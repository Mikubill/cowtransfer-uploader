package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

func addHeaders(req *http.Request) *http.Request {
	req.Header.Set("Referer", "https://cowtransfer.com/")
	req.Header.Set("User-Agent", "Chrome/80.0.3987.149 CowTransfer-Uploader")
	req.Header.Set("Origin", "https://cowtransfer.com/")
	req.Header.Set("Cookie", fmt.Sprintf("%scf-cs-k-20181214=%d;", req.Header.Get("Cookie"), time.Now().UnixNano()))
	return req
}

func addTk(req *http.Request) {
	ck := runConfig.token
	if runConfig.authCode != "" {
		ck = fmt.Sprintf("%s; cow-auth-token=%s", runConfig.token, runConfig.authCode)
	}

	req.Header.Set("cookie", ck)
	req.Header.Set("authorization", runConfig.authCode)
}

func blockPut(postURL string, buf []byte, token string) (string, error) {
	data := new(bytes.Buffer)
	data.Write(buf)
	body, err := newRequest(postURL, data, token, "PUT")
	if err != nil {
		if runConfig.debugMode {
			log.Printf("block upload failed (retrying)")
		}
		return "", err
	}
	var rBody upResp
	if err := json.Unmarshal(body, &rBody); err != nil {
		if runConfig.debugMode {
			log.Printf("block upload failed (retrying)")
		}
		return "", err
	}
	if runConfig.hashCheck {
		if hashBlock(buf) != rBody.MD5 {
			if runConfig.debugMode {
				log.Printf("block hashcheck failed (retrying)")
			}
			return "", fmt.Errorf("block hashcheck failed")
		}
		if runConfig.debugMode {
			log.Printf("hash check: %s == %s", hashBlock(buf), rBody.MD5)
		}
	}
	return rBody.Etag, nil
}

func newRequest(link string, postBody io.Reader, upToken string, action string) ([]byte, error) {
	if runConfig.debugMode {
		log.Printf("endpoint: %s", link)
	}
	client := http.Client{Timeout: time.Duration(runConfig.interval) * time.Second}
	req, err := http.NewRequest(action, link, postBody)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("build request returns error: %v", err)
		}
		return nil, err
	}
	req.Header.Set("referer", "https://cowtransfer.com/")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "UpToken "+upToken)
	if runConfig.debugMode {
		log.Println(req.Header)
	}
	resp, err := client.Do(req)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("do request returns error: %v", err)
		}
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		if runConfig.debugMode {
			log.Printf("read response returns: %v", err)
		}
		return nil, err
	}
	_ = resp.Body.Close()
	if runConfig.debugMode {
		if len(body) < 1024 {
			log.Printf("returns: %v", string(body))
		}
	}
	return body, nil
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
	addTk(req)
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
