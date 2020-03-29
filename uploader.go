package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	cmap "github.com/orcaman/concurrent-map"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	prepareSend        = "https://cowtransfer.com/transfer/preparesend"
	beforeUpload       = "https://cowtransfer.com/transfer/beforeupload"
	uploadInitEndpoint = "https://upload.qiniup.com/mkblk/%d"
	uploadFinish       = "https://cowtransfer.com/transfer/uploaded"
	uploadComplete     = "https://cowtransfer.com/transfer/complete"
	uploadMergeFile    = "https://upload.qiniup.com/mkfile/%s/key/%s/fname/%s"
	block              = 4194304
)

func upload(v string) error {
	fmt.Printf("Local: %s\n", v)
	if *debug {
		log.Println("retrieving file info...")
	}
	info, err := getFileInfo(v)
	if err != nil {
		return fmt.Errorf("getFileInfo returns error: %v", err)
	}

	config, err := getUploadConfig(info)
	if err != nil {
		return fmt.Errorf("getUploadConfig returns error: %v", err)
	}

	fmt.Printf("Destination: %s\n", config.UniqueURL)
	bar := pb.Full.Start64(info.Size())
	bar.Set(pb.Bytes, true)
	file, err := os.Open(v)
	if err != nil {
		return fmt.Errorf("openFile returns error: %v", err)
	}

	wg := new(sync.WaitGroup)
	ch := make(chan *uploadPart)
	hashMap := cmap.New()
	for i := 0; i < *parallel; i++ {
		go uploader(&ch, wg, bar, config.UploadToken, &hashMap)
	}
	part := int64(0)
	for {
		part++
		buf := make([]byte, block)
		nr, err := file.Read(buf)
		if nr <= 0 || err != nil {
			break
		}
		if nr > 0 {
			wg.Add(1)
			ch <- &uploadPart{
				content: buf[:nr],
				count:   part,
			}
		}
	}

	wg.Wait()
	close(ch)
	_ = file.Close()
	bar.Finish()
	// finish upload
	err = finishUpload(config, info, &hashMap, part)
	if err != nil {
		return fmt.Errorf("finishUpload returns error: %v", err)
	}
	return nil
}

type uploadResponse struct {
	Ticket string `json:"ctx"`
}

func uploader(ch *chan *uploadPart, wg *sync.WaitGroup, bar *pb.ProgressBar, token string, hashMap *cmap.ConcurrentMap) {
	for item := range *ch {
		client := http.Client{Timeout: time.Duration(*interval) * time.Second}
		data := new(bytes.Buffer)
		data.Write(item.content)
		postURL := fmt.Sprintf(uploadInitEndpoint, len(item.content))
		if *debug {
			log.Printf("part %d start uploading", item.count)
			log.Printf("part %d posting %s", item.count, postURL)
		}
		req, err := http.NewRequest("POST", postURL, data)
		if err != nil {
			if *debug {
				log.Printf("build request returns error: %v", err)
			}
			*ch <- item
			continue
		}
		req.Header.Set("content-type", "application/octet-stream")
		req.Header.Set("Authorization", fmt.Sprintf("UpToken %s", token))
		resp, err := client.Do(req)
		if err != nil {
			if *debug {
				log.Printf("failed uploading part %d error: %v (retrying)", item.count, err)
			}
			*ch <- item
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			if *debug {
				log.Printf("failed uploading part %d error: %v (retrying)", item.count, err)
			}
			*ch <- item
			continue
		}

		_ = resp.Body.Close()

		var rBody uploadResponse
		if err := json.Unmarshal(body, &rBody); err != nil {
			if *debug {
				log.Printf("failed uploading part %d error: %v (retrying)", item.count, err)
			}
			*ch <- item
			continue
		}
		if *debug {
			log.Printf("part %d finished. Result: %s, tk: %s", item.count, string(body), rBody.Ticket)
		}
		bar.Add(len(item.content))
		hashMap.Set(strconv.FormatInt(item.count, 10), rBody.Ticket)
		wg.Done()
	}

}

func urlSafeEncode(enc string) string {
	r := base64.StdEncoding.EncodeToString([]byte(enc))
	r = strings.ReplaceAll(r, "+", "-")
	r = strings.ReplaceAll(r, "/", "_")
	return r
}

func finishUpload(config *prepareSendResp, info os.FileInfo, hashMap *cmap.ConcurrentMap, limit int64) error {
	if *debug {
		log.Println("finishing upload...")
		log.Println("step1 -> api/mergeFile")
	}
	filename := urlSafeEncode(info.Name())
	fileLocate := urlSafeEncode(fmt.Sprintf("anonymous/%s/%s", config.TransferGUID, info.Name()))
	mergeFileURL := fmt.Sprintf(uploadMergeFile, strconv.FormatInt(info.Size(), 10), fileLocate, filename)
	postBody := ""
	for i := int64(1); i <= limit; i++ {
		item, alimasu := hashMap.Get(strconv.FormatInt(i, 10))
		if alimasu {
			postBody += item.(string) + ","
		}
	}
	if strings.HasSuffix(postBody, ",") {
		postBody = postBody[:len(postBody)-1]
	}
	if *debug {
		log.Printf("merge payload: %s\n", postBody)
	}
	_, err := newRequest(mergeFileURL, postBody, config.UploadToken)
	if err != nil {
		return err
	}

	if *debug {
		log.Println("step2 -> api/uploaded")
	}
	data := map[string]string{"transferGuid": config.TransferGUID, "fileId": ""}
	_, err = newMultipartRequest(uploadFinish, data)
	if err != nil {
		return err
	}

	if *debug {
		log.Println("step3 -> api/createVideo")
	}
	_, err = newMultipartRequest(uploadComplete, data)
	if err != nil {
		return err
	}

	return nil
}

type prepareSendResp struct {
	UploadToken  string `json:"uptoken"`
	TransferGUID string `json:"transferguid"`
	UniqueURL    string `json:"uniqueurl"`
	Prefix       string `json:"prefix"`
	QRCode       string `json:"qrcode"`
	Error        bool   `json:"error"`
	ErrorMessage string `json:"error_message"`
}

func getUploadConfig(info os.FileInfo) (*prepareSendResp, error) {

	if *debug {
		log.Println("retrieving upload config...")
		log.Println("step 1/2 -> prepareSend")
	}

	data := map[string]string{"totalSize": strconv.FormatInt(info.Size(), 10)}
	body, err := newMultipartRequest(prepareSend, data)
	if err != nil {
		return nil, err
	}
	config := new(prepareSendResp)
	err = json.Unmarshal(body, &config)
	if err != nil {
		return nil, err
	}

	// step 2
	if *debug {
		log.Println("step 2/2 -> beforeUpload")
	}
	data = map[string]string{
		"fileId":        "",
		"type":          "",
		"fileName":      info.Name(),
		"fileSize":      strconv.FormatInt(info.Size(), 10),
		"transferGuid":  config.TransferGUID,
		"storagePrefix": config.Prefix,
	}
	_, err = newMultipartRequest(beforeUpload, data)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func newRequest(link string, postBody string, upToken string) ([]byte, error) {
	if *debug {
		log.Printf("postBody: %v", postBody)
		log.Printf("endpoint: %s", link)
	}
	client := http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", link, strings.NewReader(postBody))
	if err != nil {
		if *debug {
			log.Printf("build request returns error: %v", err)
		}
		return nil, err
	}
	req.Header.Set("referer", "https://cowtransfer.com/")
	req.Header.Set("Authorization", "UpToken "+upToken)
	if *debug {
		log.Println(req.Header)
	}
	resp, err := client.Do(req)
	if err != nil {
		if *debug {
			log.Printf("do request returns error: %v", err)
		}
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if *debug {
			log.Printf("read response returns: %v", err)
		}
		return nil, err
	}
	_ = resp.Body.Close()
	if *debug {
		log.Printf("returns: %v", string(body))
	}
	return body, nil
}

func getFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func addHeaders(req *http.Request) *http.Request {
	req.Header.Set("Referer", "https://cowtransfer.com/")
	req.Header.Set("User-Agent", "Chrome/80.0.3987.149 CowTransfer-Uploader")
	req.Header.Set("Origin", "https://cowtransfer.com/")
	return req
}

func newMultipartRequest(url string, params map[string]string) ([]byte, error) {
	if *debug {
		log.Printf("postBody: %v", params)
		log.Printf("endpoint: %s", url)
	}
	client := http.Client{Timeout: 10 * time.Second}
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	_ = writer.Close()
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		if *debug {
			log.Printf("build request returns error: %v", err)
		}
		return nil, err
	}
	req.Header.Set("content-type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))
	req.Header.Set("referer", "https://cowtransfer.com/")
	req.Header.Set("cookie", *token)
	if *debug {
		log.Println(req.Header)
	}
	resp, err := client.Do(addHeaders(req))
	if err != nil {
		if *debug {
			log.Printf("do request returns error: %v", err)
		}
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if *debug {
			log.Printf("read response returns: %v", err)
		}
		return nil, err
	}
	_ = resp.Body.Close()
	if *debug {
		log.Printf("returns: %v", string(body))
	}
	if s := resp.Header.Values("Set-Cookie"); len(s) != 0 && *token == "" {
		for _, v := range s {
			ck := strings.Split(v, ";")
			*token += ck[0] + ";"
		}
		if *debug {
			log.Printf("cookies set to: %v", *token)
		}
	}

	return body, nil
}
