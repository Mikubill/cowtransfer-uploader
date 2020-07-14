package main

import (
	"encoding/json"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	downloadDetails = "https://cowtransfer.com/transfer/transferdetail?url=%s&treceive=undefined&passcode=%s"
	downloadConfig  = "https://cowtransfer.com/transfer/download?guid=%s"
)

var regex = regexp.MustCompile("[0-9a-f]{14}")

type downloadDetailsResponse struct {
	GUID         string                 `json:"guid"`
	DownloadName string                 `json:"downloadName"`
	Deleted      bool                   `json:"deleted"`
	Uploaded     bool                   `json:"uploaded"`
	Details      []downloadDetailsBlock `json:"transferFileDtos"`
}

type downloadDetailsBlock struct {
	GUID     string `json:"guid"`
	FileName string `json:"fileName"`
	Size     string `json:"size"`
}

type downloadConfigResponse struct {
	Link string `json:"link"`
}

func download(v string) error {
	fileID := regex.FindString(v)
	if fileID == "" {
		return fmt.Errorf("unknown URL format")
	}

	if runConfig.debugMode {
		log.Println("starting download...")
		log.Println("step1 -> api/getGuid")
	}
	fmt.Printf("Remote: %s\n", v)
	detailsURL := fmt.Sprintf(downloadDetails, fileID, runConfig.passCode)
	resp, err := http.Get(detailsURL)
	req, err := http.NewRequest("GET", detailsURL, nil)
	if err != nil {
		return fmt.Errorf("getDownloadDetails returns error: %s", err)
	}
	req.Header.Set("Referer", fmt.Sprintf("https://cowtransfer.com/s/%s", fileID))
	req.Header.Set("Cookie", fmt.Sprintf("cf-cs-k-20181214=%d;", time.Now().UnixNano()))
	resp, err = http.DefaultClient.Do(req)
	if err != nil { 
		return fmt.Errorf("getDownloadDetails returns error: %s", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("readDownloadDetails returns error: %s", err)
	}

	_ = resp.Body.Close()

	if runConfig.debugMode {
		log.Printf("returns: %v\n", string(body))
	}
	details := new(downloadDetailsResponse)
	if err := json.Unmarshal(body, details); err != nil {
		return fmt.Errorf("unmatshal DownloadDetails returns error: %s", err)
	}

	if details.GUID == "" {
		return fmt.Errorf("link invalid")
	}

	if details.Deleted {
		return fmt.Errorf("link deleted")
	}

	if !details.Uploaded {
		return fmt.Errorf("link not finish upload yet")
	}

	for _, item := range details.Details {
		err = downloadItem(item)
		if err != nil {
			fmt.Println(err)
		}
	}
	return nil
}

func downloadItem(item downloadDetailsBlock) error {
	if runConfig.debugMode {
		log.Println("step2 -> api/getConf")
		log.Printf("fileName: %s\n", item.FileName)
		log.Printf("fileSize: %s\n", item.Size)
		log.Printf("GUID: %s\n", item.GUID)
	}
	configURL := fmt.Sprintf(downloadConfig, item.GUID)
	req, err := http.NewRequest("POST", configURL, nil)
	if err != nil {
		return fmt.Errorf("createRequest returns error: %s, onfile: %s", err, item.FileName)
	}
	resp, err := http.DefaultClient.Do(addHeaders(req))
	if err != nil {
		return fmt.Errorf("getDownloadConfig returns error: %s, onfile: %s", err, item.FileName)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("readDownloadConfig returns error: %s, onfile: %s", err, item.FileName)
	}

	_ = resp.Body.Close()
	if runConfig.debugMode {
		log.Printf("returns: %v\n", string(body))
	}
	config := new(downloadConfigResponse)
	if err := json.Unmarshal(body, config); err != nil {
		return fmt.Errorf("unmatshal DownloadConfig returns error: %s, onfile: %s", err, item.FileName)
	}

	if runConfig.debugMode {
		log.Println("step3 -> startDownload")
	}
	filePath := runConfig.prefix

	if isExist(runConfig.prefix) {
		if isFile(runConfig.prefix) {
			filePath = runConfig.prefix
		} else {
			filePath = path.Join(runConfig.prefix, item.FileName)
		}
	}

	fmt.Printf("File save to: %s\n", filePath)
	numSize, err := strconv.ParseFloat(item.Size, 10)
	if err != nil {
		return fmt.Errorf("failed Parsing with error: %s, onfile: %s", err, item.FileName)
	}
	var bar *pb.ProgressBar
	if !runConfig.silentMode {
		bar = pb.Full.Start64(int64(numSize * 1024))
		bar.Set(pb.Bytes, true)
	}
	err = downloadFile(filePath, config.Link, bar)
	if !runConfig.silentMode && bar != nil {
		bar.Finish()
	}
	if err != nil {
		return fmt.Errorf("failed DownloadConfig with error: %s, onfile: %s", err, item.FileName)
	}
	return nil
}

type writeCounter struct {
	bar    *pb.ProgressBar
	offset int64
	writer *os.File
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n, err := wc.writer.WriteAt(p, wc.offset)
	if err != nil {
		return 0, err
	}
	wc.offset += int64(n)
	if !runConfig.silentMode && wc.bar != nil {
		wc.bar.Add(n)
	}
	return n, nil
}

func downloadFile(filepath string, url string, bar *pb.ProgressBar) error {

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return err
	}
	addHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode > 400 {
		return fmt.Errorf("link unavailable, %s", resp.Status)
	}
	length, err := strconv.ParseInt(resp.Header.Get("content-length"), 10, 64)
	if err != nil {
		return err
	}
	if !runConfig.silentMode && bar != nil {
		bar.SetTotal(length)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	//counter := &writeCounter{bar: bar}
	//_, err = io.Copy(ioutil.Discard, io.TeeReader(resp.Body, counter))
	_parallel := 1
	if err := out.Truncate(length); err != nil {
		return fmt.Errorf("tmpfile fruncate failed: %s", err)
	}
	if length > 10*1024*1024 && resp.Header.Get("Accept-Ranges") != "" && runConfig.parallel > 1 {
		_parallel = runConfig.parallel
	}

	wg := new(sync.WaitGroup)
	blk := length / int64(_parallel)

	if runConfig.debugMode {
		log.Printf("filesize = %d", length)
		log.Printf("parallel = %d", _parallel)
		log.Printf("block = %d", blk)
	}
	for i := 0; i <= _parallel; i++ {
		wg.Add(1)
		start := int64(i) * blk
		end := start + blk
		ranger := fmt.Sprintf("%d-%d", start, end)
		if end >= length {
			ranger = fmt.Sprintf("%d-%d", start, length)
		}
		if runConfig.debugMode {
			log.Printf("downloading parallel = %d\n", i)
			log.Printf("selecting block = %d\n", blk)
			log.Printf("using range = %s\n", ranger)
		}
		go func() {
			counter := &writeCounter{bar: bar, offset: start, writer: out}
			for {
				err = parallelDownloader(ranger, url, counter, wg)
				if err == nil {
					break
				}
			}
		}()
	}
	wg.Wait()

	fmt.Print("\n")
	return nil
}

func parallelDownloader(ranger string, url string, counter *writeCounter, wg *sync.WaitGroup) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("createRequest error: %s\n", err)
	}
	req.Header.Set("Range", "bytes="+ranger)
	addHeaders(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("doRequest error: %s\n", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	_, err = io.Copy(ioutil.Discard, io.TeeReader(resp.Body, counter))
	if err != nil {
		return fmt.Errorf("parallel bytes copy returns: %s", err)
	}
	wg.Done()
	return nil
}

func isExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		if os.IsNotExist(err) {
			return false
		}
		//log.Println(err)
		return false
	}
	return true
}

func isDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func isFile(path string) bool {
	return !isDir(path)
}
