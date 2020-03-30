package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

var (
	token    = new(string)
	parallel = new(int)
	interval = new(int)
	prefix   = new(string)
	debug    = new(bool)
	single   = new(bool)
	version  = new(bool)
	keep     = new(bool)
	blockSize = new(int)
	hashCheck = new(bool)
	build    string
)

type uploadPart struct {
	content []byte
	count   int64
}

func init() {
	addFlag(hashCheck, []string{"-hash", "-r", "--hash"}, false, "Check Hash after block upload (might slower)")
	addFlag(token, []string{"-cookie", "-c", "--cookie"}, "", "Your User cookie (optional)")
	addFlag(parallel, []string{"-parallel", "-p", "--parallel"}, 4, "Parallel task count (default 4)")
	addFlag(blockSize, []string{"-block", "-b", "--block"}, 262144, "Upload Block Size (default 262144)")
	addFlag(interval, []string{"-timeout", "-t", "--timeout"}, 10, "Request retry/timeout limit (in second, default 10)")
	addFlag(prefix, []string{"-prefix", "-o", "--output"}, ".", "File download dictionary/name (default \".\")")
	addFlag(single, []string{"-single", "-s", "--single"}, false, "Single Upload Mode")
	addFlag(debug, []string{"-verbose", "-v", "--verbose"}, false, "Verbose Mode")
	addFlag(keep, []string{"-keep", "-k", "--keep"}, false, "Keep program active when upload finish")
	addFlag(version, []string{"-version", "--version"}, false, "Print version and exit")

	flag.Usage = printUsage
	flag.Parse()
}

func main() {
	files := flag.Args()

	if *version {
		printVersion()
		return
	}

	if *debug {
		log.Printf("cookie = %s", *token)
		log.Printf("block size = %d", block)
		log.Printf("verbose = true")
		log.Printf("single = %v", *single)
		log.Printf("timeout = %d", *interval)
		log.Printf("parallel = %d", *parallel)
		log.Printf("files = %s", files)
	}
	if len(files) == 0 {
		fmt.Printf("missing file(s) or url(s)\n")
		printUsage()
		return
	}
	if *blockSize > 4194304 {
		*blockSize = 524288
	}

	var f []string
	for _, v := range files {
		var err error
		if strings.HasPrefix(v, "https://") {
			// Download Mode
			err = download(v)
		} else {
			f = append(f, v)
		}
		if err != nil {
			fmt.Printf("Error: %v", err)
		}
	}
	upload(f)

	if *keep {
		fmt.Print("Press the enter key to exit...")
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')
	}
}

var commands [][]string

func printUsage() {
	fmt.Printf("\nUsage:\n\n  %s [options] file(s)/url(s)\n\n", os.Args[0])
	fmt.Printf("Options:\n\n")
	for _, val := range commands {
		s := fmt.Sprintf("  %s %s", val[0], val[1])
		block := strings.Repeat(" ", 30-len(s))
		fmt.Printf("%s%s%s\n", s, block, val[2])
	}
	fmt.Printf("\n")
}

func printVersion() {
	version := fmt.Sprintf("\ncowTransfer-uploader\n"+
		"Source: https://github.com/Mikubill/cowtransfer-uploader\n"+
		"Build: %s\n", build)
	fmt.Println(version)
}

func addFlag(p interface{}, cmd []string, val interface{}, usage string) {
	s := []string{strings.Join(cmd[1:], ", "), "", usage}
	for _, item := range cmd {
		switch p.(type) {
		case *int:
			s[1] = "int"
			*p.(*int) = val.(int)
			flag.IntVar(p.(*int), item[1:], val.(int), usage)
		case *string:
			s[1] = "string"
			*p.(*string) = val.(string)
			flag.StringVar(p.(*string), item[1:], val.(string), usage)
		case *bool:
			*p.(*bool) = val.(bool)
			flag.BoolVar(p.(*bool), item[1:], val.(bool), usage)
		}
	}
	commands = append(commands, s)
}
