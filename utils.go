package main

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func hashBlock(buf []byte) string {
	return fmt.Sprintf("%x", md5.Sum(buf))
}

func urlSafeEncode(enc string) string {
	r := base64.StdEncoding.EncodeToString([]byte(enc))
	r = strings.ReplaceAll(r, "+", "-")
	r = strings.ReplaceAll(r, "/", "_")
	return r
}

func getFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return info, nil
}
