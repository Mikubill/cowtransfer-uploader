package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"unsafe"
)

var (
	runConfig = new(mainConfig)
	build     string
	commands  [][]string
)

type uploadPart struct {
	content []byte
	count   int64
}

func init() {
	addFlag(&runConfig.token, []string{"-cookie", "-c", "--cookie"}, "", "Your User cookie (optional)")
	addFlag(&runConfig.parallel, []string{"-parallel", "-p", "--parallel"}, 4, "Parallel task count (default 4)")
	addFlag(&runConfig.blockSize, []string{"-block", "-b", "--block"}, 262144, "Upload Block Size (default 262144)")
	addFlag(&runConfig.interval, []string{"-timeout", "-t", "--timeout"}, 10, "Request retry/timeout limit (in second, default 10)")
	addFlag(&runConfig.prefix, []string{"-prefix", "-o", "--output"}, ".", "File download dictionary/name (default \".\")")
	addFlag(&runConfig.singleMode, []string{"-single", "-s", "--single"}, false, "Single Upload Mode")
	addFlag(&runConfig.debugMode, []string{"-verbose", "-v", "--verbose"}, false, "Verbose Mode")
	addFlag(&runConfig.keepMode, []string{"-keep", "-k", "--keep"}, false, "Keep program active when upload finish")
	addFlag(&runConfig.hashCheck, []string{"-hash", "--hash"}, false, "Check Hash after block upload (might slower)")
	addFlag(&runConfig.passCode, []string{"-password", "--password"}, "", "Set password")
	addFlag(&runConfig.version, []string{"-version", "--version"}, false, "Print version and exit")

	flag.Usage = printUsage
	flag.Parse()
}

func main() {
	files := flag.Args()

	if runConfig.version {
		printVersion()
		return
	}

	if runConfig.debugMode {
		log.Printf("config = %+v", runConfig)
		log.Printf("files = %s", files)
	}
	if len(files) == 0 {
		fmt.Printf("missing file(s) or url(s)\n")
		printUsage()
		return
	}
	if runConfig.blockSize > 4194304 {
		runConfig.blockSize = 524288
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
	if len(f) != 0 {
		upload(f)
	}

	if runConfig.keepMode {
		fmt.Print("Press the enter key to exit...")
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')
	}
}

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
	ptr := unsafe.Pointer(reflect.ValueOf(p).Pointer())
	for _, item := range cmd {
		switch val.(type) {
		case int:
			s[1] = "int"
			flag.IntVar((*int)(ptr), item[1:], val.(int), usage)
		case string:
			s[1] = "string"
			flag.StringVar((*string)(ptr), item[1:], val.(string), usage)
		case bool:
			flag.BoolVar((*bool)(ptr), item[1:], val.(bool), usage)
		}
	}
	commands = append(commands, s)
}
