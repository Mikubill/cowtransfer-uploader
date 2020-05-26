package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	cmap "github.com/orcaman/concurrent-map"
	"hash/crc32"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	prepareSend        = "https://cowtransfer.com/transfer/preparesend"
	setPassword        = "https://cowtransfer.com/transfer/v2/bindpasscode"
	beforeUpload       = "https://cowtransfer.com/transfer/beforeupload"
	uploadInitEndpoint = "https://upload.qiniup.com/mkblk/%d"
	uploadEndpoint     = "https://upload.qiniup.com/bput/%s/%d"
	uploadFinish       = "https://cowtransfer.com/transfer/uploaded"
	uploadComplete     = "https://cowtransfer.com/transfer/complete"
	uploadMergeFile    = "https://upload.qiniup.com/mkfile/%s/key/%s/fname/%s"
	block              = 4194304
)

func upload(files []string) {
	if !runConfig.singleMode {
		for _, v := range files {
			if isExist(v) {
				err := filepath.Walk(v, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						fmt.Printf("filapath walker returns error: %v, onfile: %s", err, path)
						return nil
					}
					if info.IsDir() {
						return nil
					}
					config, err := getSendConfig(info.Size())
					if err != nil {
						fmt.Printf("getSendConfig returns error: %v, onfile: %s", err, path)
						return nil
					}
					fmt.Printf("Destination: %s\n", config.UniqueURL)
					err = _upload(path, config)
					if err != nil {
						fmt.Printf("upload returns error: %v, onfile: %s", err, path)
					}
					err = completeUpload(config)
					if err != nil {
						fmt.Printf("complete upload returns error: %v, onfile: %s", err, path)
					}
					return nil
				})
				if err != nil {
					fmt.Printf("filepath.walk returns error: %v, onfile: %s", err, v)
				}
			} else {
				fmt.Printf("%s not found\n", v)
			}
		}

		return
	}
	totalSize := int64(0)

	for _, v := range files {
		if isExist(v) {
			err := filepath.Walk(v, func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}
				totalSize += info.Size()
				return nil
			})
			if err != nil {
				fmt.Printf("filepath.walk returns error: %v, onfile: %s\n", err, v)
			}
		} else {
			fmt.Printf("%s not found\n", v)
		}
	}

	config, err := getSendConfig(totalSize)
	if err != nil {
		fmt.Printf("getSendConfig(single mode) returns error: %v\n", err)
	}
	fmt.Printf("Destination: %s\n", config.UniqueURL)
	for _, v := range files {
		if isExist(v) {
			err = filepath.Walk(v, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					fmt.Printf("filapath walker returns error: %v, onfile: %s", err, path)
					return nil
				}
				if info.IsDir() {
					return nil
				}
				err = _upload(path, config)
				if err != nil {
					fmt.Printf("upload returns error: %v, onfile: %s\n", err, path)
				}
				return nil
			})
			if err != nil {
				fmt.Printf("filepath.walk(upload) returns error: %v, onfile: %s\n", err, v)
			}
		} else {
			fmt.Printf("%s not found\n", v)
		}
	}
	err = completeUpload(config)
	if err != nil {
		fmt.Printf("complete upload(single mode) returns error: %v\n", err)
	}
}

func _upload(v string, baseConf *prepareSendResp) error {
	fmt.Printf("Local: %s\n", v)
	if runConfig.debugMode {
		log.Println("retrieving file info...")
	}
	info, err := getFileInfo(v)
	if err != nil {
		return fmt.Errorf("getFileInfo returns error: %v", err)
	}

	config, err := getUploadConfig(info, baseConf)
	if err != nil {
		return fmt.Errorf("getUploadConfig returns error: %v", err)
	}
	var bar *pb.ProgressBar
	if !runConfig.silentMode {
		bar = pb.Full.Start64(info.Size())
		bar.Set(pb.Bytes, true)
	}
	file, err := os.Open(v)
	if err != nil {
		return fmt.Errorf("openFile returns error: %v", err)
	}

	wg := new(sync.WaitGroup)
	ch := make(chan *uploadPart)
	hashMap := cmap.New()
	for i := 0; i < runConfig.parallel; i++ {
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
	if !runConfig.silentMode && bar != nil{
		bar.Finish()
	}
	// finish upload
	err = finishUpload(config, info, &hashMap, part)
	if err != nil {
		return fmt.Errorf("finishUpload returns error: %v", err)
	}
	return nil
}

func uploader(ch *chan *uploadPart, wg *sync.WaitGroup, bar *pb.ProgressBar, token string, hashMap *cmap.ConcurrentMap) {
	for item := range *ch {
		postURL := fmt.Sprintf(uploadInitEndpoint, len(item.content))
		if runConfig.debugMode {
			log.Printf("part %d start uploading, size: %d", item.count, len(item.content))
			log.Printf("part %d posting %s", item.count, postURL)
		}

		// makeBlock
		body, err := newRequest(postURL, nil, token, 0)
		if err != nil {
			if runConfig.debugMode {
				log.Printf("failed make mkblk on part %d, error: %s (retrying)",
					item.count, err)
			}
			*ch <- item
			continue
		}
		var rBody uploadResponse
		if err := json.Unmarshal(body, &rBody); err != nil {
			if runConfig.debugMode {
				log.Printf("failed make mkblk on part %d error: %v, returns: %s (retrying)",
					item.count, string(body), strings.ReplaceAll(err.Error(), "\n", ""))
			}
			*ch <- item
			continue
		}

		//blockPut
		failFlag := false
		blockCount := int(math.Ceil(float64(len(item.content)) / float64(runConfig.blockSize)))
		if runConfig.debugMode {
			log.Printf("init: part %d block %d ", item.count, blockCount)
		}
		ticket := rBody.Ticket
		for i := 0; i < blockCount; i++ {
			start := i * runConfig.blockSize
			end := (i + 1) * runConfig.blockSize
			var buf []byte
			if end > len(item.content) {
				buf = item.content[start:]
			} else {
				buf = item.content[start:end]
			}
			if runConfig.debugMode {
				log.Printf("part %d block %d [%d:%d] start upload...", item.count, i, start, end)
			}
			postURL = fmt.Sprintf(uploadEndpoint, ticket, start)
			ticket, err = blockPut(postURL, buf, token, 0)
			if err != nil {
				if runConfig.debugMode {
					log.Printf("part %d block %d failed. error: %s (retrying)", item.count, i, err)
				}
				failFlag = true
				break
			}
			if !runConfig.silentMode && bar != nil {
				bar.Add(len(buf))
			}
		}
		if failFlag {
			*ch <- item
			continue
		}

		if runConfig.debugMode {
			log.Printf("part %d finished.", item.count)
		}
		hashMap.Set(strconv.FormatInt(item.count, 10), ticket)
		wg.Done()
	}

}

func blockPut(postURL string, buf []byte, token string, retry int) (string, error) {
	data := new(bytes.Buffer)
	data.Write(buf)
	body, err := newRequest(postURL, data, token, 0)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("block upload failed (retrying)")
		}
		if retry > 3 {
			return "", err
		}
		return blockPut(postURL, buf, token, retry+1)
	}
	var rBody uploadResponse
	if err := json.Unmarshal(body, &rBody); err != nil {
		if runConfig.debugMode {
			log.Printf("block upload failed (retrying)")
		}
		if retry > 3 {
			return "", err
		}
		return blockPut(postURL, buf, token, retry+1)
	}
	if runConfig.hashCheck {
		if hashBlock(buf) != rBody.Hash {
			if runConfig.debugMode {
				log.Printf("block hashcheck failed (retrying)")
			}
			if retry > 3 {
				return "", err
			}
			return blockPut(postURL, buf, token, retry+1)
		}
	}
	return rBody.Ticket, nil
}

func hashBlock(buf []byte) int {
	return int(crc32.ChecksumIEEE(buf))
}

func urlSafeEncode(enc string) string {
	r := base64.StdEncoding.EncodeToString([]byte(enc))
	r = strings.ReplaceAll(r, "+", "-")
	r = strings.ReplaceAll(r, "/", "_")
	return r
}

func finishUpload(config *prepareSendResp, info os.FileInfo, hashMap *cmap.ConcurrentMap, limit int64) error {
	if runConfig.debugMode {
		log.Println("finishing upload...")
		log.Println("step1 -> api/mergeFile")
	}
	filename := urlSafeEncode(info.Name())
	var fileLocate string
	fileLocate = urlSafeEncode(fmt.Sprintf("%s/%s/%s", config.Prefix, config.TransferGUID, info.Name()))
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
	if runConfig.debugMode {
		log.Printf("merge payload: %s\n", postBody)
	}
	reader := bytes.NewReader([]byte(postBody))
	_, err := newRequest(mergeFileURL, reader, config.UploadToken, 0)
	if err != nil {
		return err
	}

	if runConfig.debugMode {
		log.Println("step2 -> api/uploaded")
	}
	data := map[string]string{"transferGuid": config.TransferGUID, "fileId": ""}
	body, err := newMultipartRequest(uploadFinish, data, 0)
	if err != nil {
		return err
	}
	if string(body) != "true" {
		return fmt.Errorf("finish upload failed: status != true")
	}
	return nil
}

func completeUpload(config *prepareSendResp) error {
	data := map[string]string{"transferGuid": config.TransferGUID, "fileId": ""}
	if runConfig.debugMode {
		log.Println("step3 -> api/completeUpload")
	}
	body, err := newMultipartRequest(uploadComplete, data, 0)
	if err != nil {
		return err
	}
	var rBody finishResponse
	if err := json.Unmarshal(body, &rBody); err != nil {
		return fmt.Errorf("read finish resp failed: %s", err)
	}
	if !rBody.Status {
		return fmt.Errorf("finish upload failed: complete is not true")
	}
	fmt.Printf("Short Download Code: %s\n", rBody.TempDownloadCode)
	return nil
}

func getSendConfig(totalSize int64) (*prepareSendResp, error) {
	data := map[string]string{
		"totalSize": strconv.FormatInt(totalSize, 10),
	}
	body, err := newMultipartRequest(prepareSend, data, 0)
	if err != nil {
		return nil, err
	}
	config := new(prepareSendResp)
	err = json.Unmarshal(body, &config)
	if err != nil {
		return nil, err
	}
	if config.Error {
		return nil, fmt.Errorf(config.ErrorMessage)
	}
	if runConfig.passCode != "" {
		// set password
		data := map[string]string{
			"transferguid": config.TransferGUID,
			"passcode":     runConfig.passCode,
		}
		body, err = newMultipartRequest(setPassword, data, 0)
		if err != nil {
			return nil, err
		}
		if string(body) != "true" {
			return nil, fmt.Errorf("set password unsuccessful")
		}
	}
	return config, nil
}

func getUploadConfig(info os.FileInfo, config *prepareSendResp) (*prepareSendResp, error) {

	if runConfig.debugMode {
		log.Println("retrieving upload config...")
		log.Println("step 2/2 -> beforeUpload")
	}

	data := map[string]string{
		"fileId":        "",
		"type":          "",
		"fileName":      info.Name(),
		"originalName":      info.Name(),
		"fileSize":      strconv.FormatInt(info.Size(), 10),
		"transferGuid":  config.TransferGUID,
		"storagePrefix": config.Prefix,
	}
	_, err := newMultipartRequest(beforeUpload, data, 0)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func newRequest(link string, postBody io.Reader, upToken string, retry int) ([]byte, error) {
	if runConfig.debugMode {
		log.Printf("endpoint: %s", link)
	}
	client := http.Client{Timeout: time.Duration(runConfig.interval) * time.Second}
	req, err := http.NewRequest("POST", link, postBody)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("build request returns error: %v", err)
		}
		if retry > 3 {
			return nil, err
		}
		return newRequest(link, postBody, upToken, retry+1)
	}
	req.Header.Set("referer", "https://cowtransfer.com/")
	req.Header.Set("Authorization", "UpToken "+upToken)
	if runConfig.debugMode {
		log.Println(req.Header)
	}
	resp, err := client.Do(req)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("do request returns error: %v", err)
		}
		if retry > 3 {
			return nil, err
		}
		return newRequest(link, postBody, upToken, retry+1)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("read response returns: %v", err)
		}
		if retry > 3 {
			return nil, err
		}
		return newRequest(link, postBody, upToken, retry+1)
	}
	_ = resp.Body.Close()
	if runConfig.debugMode {
		if len(body) < 1024 {
			log.Printf("returns: %v", string(body))
		}
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
	req.Header.Set("Cookie", fmt.Sprintf("%scf-cs-k-20181214=%d;", req.Header.Get("Cookie"), time.Now().UnixNano()))
	return req
}

func newMultipartRequest(url string, params map[string]string, retry int) ([]byte, error) {
	if runConfig.debugMode {
		log.Printf("retrying: %v", retry)
		log.Printf("postBody: %v", params)
		log.Printf("endpoint: %s", url)
	}
	client := http.Client{Timeout: time.Duration(runConfig.interval) * time.Second}
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	for key, val := range params {
		_ = writer.WriteField(key, val)
	}
	_ = writer.Close()
	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("build request returns error: %v", err)
		}
		if retry > 3 {
			return nil, err
		}
		return newMultipartRequest(url, params, retry+1)
	}
	req.Header.Set("content-type", fmt.Sprintf("multipart/form-data;boundary=%s", writer.Boundary()))
	req.Header.Set("referer", "https://cowtransfer.com/")
	req.Header.Set("cookie", runConfig.token)
	if runConfig.debugMode {
		log.Println(req.Header)
	}
	resp, err := client.Do(addHeaders(req))
	if err != nil {
		if runConfig.debugMode {
			log.Printf("do request returns error: %v", err)
		}
		if retry > 3 {
			return nil, err
		}
		return newMultipartRequest(url, params, retry+1)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("read response returns: %v", err)
		}
		if retry > 3 {
			return nil, err
		}
		return newMultipartRequest(url, params, retry+1)
	}
	_ = resp.Body.Close()
	if runConfig.debugMode {
		log.Printf("returns: %v", string(body))
	}
	if s := resp.Header.Values("Set-Cookie"); len(s) != 0 && runConfig.token == "" {
		for _, v := range s {
			ck := strings.Split(v, ";")
			runConfig.token += ck[0] + ";"
		}
		if runConfig.debugMode {
			log.Printf("cookies set to: %v", runConfig.token)
		}
	}

	return body, nil
}
