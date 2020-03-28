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
	"strconv"
)

const (
	downloadDetails = "https://cowtransfer.com/transfer/transferdetail?url=%s&treceive=undefined"
	downloadConfig  = "https://cowtransfer.com/transfer/download?guid=%s"
)

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
	Size     string  `json:"size"`
}

type downloadConfigResponse struct {
	Link string `json:"link"`
}

func download(v string) error {
	fileID := regex.FindString(v)
	if fileID == "" {
		return fmt.Errorf("unknown URL format")
	}

	if *debug {
		log.Println("starting download...")
		log.Println("step1 -> api/getGuid")
	}
	fmt.Printf("Remote: %s\n", v)
	detailsURL := fmt.Sprintf(downloadDetails, fileID)
	resp, err := http.Get(detailsURL)
	if err != nil {
		return fmt.Errorf("getDownloadDetails returns error: %s", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("readDownloadDetails returns error: %s", err)
	}

	_ = resp.Body.Close()

	if *debug {
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

	if !isExist(*prefix) {
		err = os.MkdirAll(*prefix, 00666)
		if err != nil {
			return fmt.Errorf("createFolder returns error: %s", err)
		}
	}

	for _, item := range details.Details {
		if *debug {
			log.Println("step2 -> api/getConf")
			log.Printf("fileName: %s\n", item.FileName)
			log.Printf("fileSize: %s\n", item.Size)
			log.Printf("GUID: %s\n", item.GUID)
		}
		configURL := fmt.Sprintf(downloadConfig, item.GUID)
		req, err := http.NewRequest("POST", configURL, nil)
		if err != nil {
			fmt.Printf("createRequest returns error: %s, onfile: %s", err, item.FileName)
			continue
		}
		resp, err = http.DefaultClient.Do(addHeaders(req))
		if err != nil {
			fmt.Printf("getDownloadConfig returns error: %s, onfile: %s", err, item.FileName)
			continue
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("readDownloadConfig returns error: %s, onfile: %s", err, item.FileName)
			continue
		}

		_ = resp.Body.Close()
		if *debug {
			log.Printf("returns: %v\n", string(body))
		}
		config := new(downloadConfigResponse)
		if err := json.Unmarshal(body, config); err != nil {
			fmt.Printf("unmatshal DownloadConfig returns error: %s, onfile: %s", err, item.FileName)
			continue
		}

		if *debug {
			log.Println("step3 -> startDownload")
		}
		filePath := path.Join(*prefix, item.FileName)
		fmt.Printf("File save to: %s\n", filePath)
		numSize, err := strconv.ParseFloat(item.Size, 10)
		if err != nil {
			fmt.Printf("failed Parsing with error: %s, onfile: %s", err, item.FileName)
			continue
		}
		bar := pb.Full.Start64(int64(numSize * 1024))
		bar.Set(pb.Bytes, true)
		err = downloadFile(filePath, config.Link, bar)
		bar.Finish()
		if err != nil {
			fmt.Printf("failed DownloadConfig with error: %s, onfile: %s", err, item.FileName)
			continue
		}
	}
	return nil
}

type writeCounter struct {
	bar *pb.ProgressBar
}

func (wc *writeCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.bar.Add(n)
	return n, nil
}

func downloadFile(filepath string, url string, bar *pb.ProgressBar) error {
	out, err := os.Create(filepath + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	counter := &writeCounter{bar: bar}
	_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	if err != nil {
		return err
	}

	fmt.Print("\n")

	err = os.Rename(filepath+".tmp", filepath)
	if err != nil {
		return err
	}

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
