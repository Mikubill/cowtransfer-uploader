package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

var (
	token    = flag.String("cookie", "", "Your User cookie (optional)")
	parallel = flag.Int("parallel", 4, "parallel task count")
	interval = flag.Int("timeout", 30, "request retry/timeout limit")
	prefix   = flag.String("prefix", ".", "file download prefix")
	debug    = flag.Bool("verbose", false, "Verbose Mode")
	regex    = regexp.MustCompile("[0-9a-f]{14}")
)

type uploadPart struct {
	content []byte
	count   int64
}

func main() {
	flag.Usage = printUsage
	flag.Parse()
	files := flag.Args()

	if *debug {
		log.Printf("cookie = %s", *token)
		log.Printf("block size = %d", block)
		log.Printf("verbose = true")
		log.Printf("timeout = %d", *interval)
		log.Printf("parallel = %d", *parallel)
		log.Printf("files = %s", files)
	}
	if len(files) == 0 {
		fmt.Printf("missing file(s)\n\n")
		printUsage()
		return
	}

	for _, v := range files {
		var err error
		if strings.HasPrefix(v, "https://") {
			// Download Mode
			err = download(v)
		} else {
			err = upload(v)
		}
		if err != nil {
			fmt.Println(err)
		}

	}
}

func printUsage() {
	fmt.Printf("Usage:\n\n  %s [options] file(s)/url(s)\n\n", os.Args[0])
	fmt.Printf("Options:\n\n")
	flag.PrintDefaults()
}
