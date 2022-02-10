package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/cheggaaa/pb/v3"
	cmap "github.com/orcaman/concurrent-map"
)

const (
	prepareSend    = "https://cowtransfer.com/api/transfer/v2/preparesend"
	setPassword    = "https://cowtransfer.com/api/transfer/v2/bindpasscode"
	beforeUpload   = "https://cowtransfer.com/api/transfer/v2/beforeupload"
	uploadFinish   = "https://cowtransfer.com/api/transfer/v2/uploaded"
	uploadComplete = "https://cowtransfer.com/api/transfer/v2/complete"
	initUpload     = "https://upload.qiniup.com/buckets/cftransfer/objects/%s/uploads"
	doUpload       = "https://upload.qiniup.com/buckets/cftransfer/objects/%s/uploads/%s/%d"
	finUpload      = "https://upload.qiniup.com/buckets/cftransfer/objects/%s/uploads/%s"

	// block = 1024 * 1024
)

type uploadConfig struct {
	wg      *sync.WaitGroup
	config  *initResp
	hashMap *cmap.ConcurrentMap
}

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
		//go uploader(&ch, wg, bar, config.UploadToken, &hashMap)
		go uploader(&ch, uploadConfig{
			wg:      wg,
			config:  config,
			hashMap: &hashMap,
		})
	}
	part := int64(0)
	for {
		part++
		buf := make([]byte, runConfig.blockSize)
		nr, err := file.Read(buf)
		if nr <= 0 || err != nil {
			break
		}
		if nr > 0 {
			wg.Add(1)
			ch <- &uploadPart{
				bar:     bar,
				content: buf[:nr],
				count:   part,
			}
		}
	}

	wg.Wait()
	close(ch)
	_ = file.Close()
	if !runConfig.silentMode && bar != nil {
		bar.Finish()
	}
	// finish upload
	err = finishUpload(config, info, &hashMap, part)
	if err != nil {
		return fmt.Errorf("finishUpload returns error: %v", err)
	}
	return nil
}

func uploader(ch *chan *uploadPart, conf uploadConfig) {
	for item := range *ch {
	Start:
		postURL := fmt.Sprintf(doUpload, conf.config.EncodeID, conf.config.ID, item.count)
		if runConfig.debugMode {
			log.Printf("part %d start uploading, size: %d", item.count, len(item.content))
			log.Printf("part %d posting %s", item.count, postURL)
		}

		//blockPut
		ticket, err := blockPut(postURL, item.content, conf.config.Token)
		if err != nil {
			if runConfig.debugMode {
				log.Printf("part %d failed. error: %s", item.count, err)
			}
			goto Start
		}
		if !runConfig.silentMode && item.bar != nil {
			item.bar.Add(len(item.content))
		}

		if runConfig.debugMode {
			log.Printf("part %d finished.", item.count)
		}
		conf.hashMap.Set(strconv.FormatInt(item.count, 10), ticket)
		conf.wg.Done()
	}

}

func finishUpload(config *initResp, info os.FileInfo, hashMap *cmap.ConcurrentMap, limit int64) error {
	if runConfig.debugMode {
		log.Println("finishing upload...")
		log.Println("step1 -> api/mergeFile")
	}
	// filename := urlSafeEncode(info.Name())
	// var fileLocate string
	// fileLocate = urlSafeEncode(fmt.Sprintf("%s/%s/%s", config.Prefix, config.TransferGUID, info.Name()))
	// mergeFileURL := fmt.Sprintf(uploadMergeFile, strconv.FormatInt(info.Size(), 10), fileLocate, filename)
	mergeFileURL := fmt.Sprintf(finUpload, config.EncodeID, config.ID)
	var postData clds
	for i := int64(1); i <= limit; i++ {
		item, alimasu := hashMap.Get(strconv.FormatInt(i, 10))
		if alimasu {
			postData.Parts = append(postData.Parts, slek{
				ETag: item.(string),
				Part: i,
			})
		}
	}
	postData.FName = info.Name()
	postBody, err := json.Marshal(postData)
	if err != nil {
		return err
	}
	if runConfig.debugMode {
		log.Printf("merge payload: %s\n", postBody)
	}
	reader := bytes.NewReader(postBody)
	resp, err := newRequest(mergeFileURL, reader, config.Token, "POST")
	if err != nil {
		return err
	}

	// read returns
	var mergeResp *uploadResult
	if err = json.Unmarshal(resp, &mergeResp); err != nil {
		return err
	}

	if runConfig.debugMode {
		log.Println("step2 -> api/uploaded")
	}
	data := map[string]string{
		"transferGuid": config.TransferGUID,
		"fileGuid":     config.FileGUID,
		"hash":         mergeResp.Hash,
	}
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
		"validDays": strconv.Itoa(runConfig.validDays),
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

func getUploadConfig(info os.FileInfo, config *prepareSendResp) (*initResp, error) {

	if runConfig.debugMode {
		log.Println("retrieving upload config...")
		log.Println("step 2/2 -> beforeUpload")
	}

	data := map[string]string{
		"fileId":        "",
		"type":          "",
		"fileName":      info.Name(),
		"originalName":  info.Name(),
		"fileSize":      strconv.FormatInt(info.Size(), 10),
		"transferGuid":  config.TransferGUID,
		"storagePrefix": config.Prefix,
	}
	resp, err := newMultipartRequest(beforeUpload, data, 0)
	if err != nil {
		return nil, err
	}
	var beforeResp *beforeSendResp
	if err = json.Unmarshal(resp, &beforeResp); err != nil {
		return nil, err
	}
	config.FileGUID = beforeResp.FileGuid

	data = map[string]string{
		"transferGuid":  config.TransferGUID,
		"storagePrefix": config.Prefix,
	}
	p, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	w := urlSafeEncode(fmt.Sprintf("%s/%s/%s", config.Prefix, config.TransferGUID, info.Name()))
	inits := fmt.Sprintf(initUpload, w)
	resp, err = newRequest(inits, bytes.NewReader(p), config.UploadToken, "POST")
	if err != nil {
		return nil, err
	}
	var initResp *initResp
	if err = json.Unmarshal(resp, &initResp); err != nil {
		return nil, err
	}
	initResp.Token = config.UploadToken
	initResp.EncodeID = w
	initResp.TransferGUID = config.TransferGUID
	initResp.FileGUID = config.FileGUID

	// return config, nil
	return initResp, nil
}
