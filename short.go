package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	verifyDownloadCode = "https://cowtransfer.com/api/transfer/verifydownloadcode?code="
)

func translate(code string) (string, error) {
	resp, err := http.Get(verifyDownloadCode + code)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		return "", fmt.Errorf("abnormal http return code: %d", resp.StatusCode)
	}

	if contentType := resp.Header.Get("Content-Type"); !strings.Contains(contentType, "json") {
		return "", fmt.Errorf("unrecognized content type: %s", contentType)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if 0 == len(data) {
		return "", fmt.Errorf("data length is not as expected")
	}

	result := struct {
		Url string `json:"url"`
	}{}

	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	if "" == result.Url {
		return "", fmt.Errorf("not found or out of date")
	}

	return "https://cowtransfer.com/s/" + result.Url, nil
}
