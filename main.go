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

	"github.com/cheggaaa/pb/v3"
)

var (
	runConfig = new(mainConfig)
	build     string
	commands  [][]string
)

type uploadPart struct {
	content []byte
	count   int64
	bar     *pb.ProgressBar
}

func init() {
	addFlag(&runConfig.authCode, []string{"auth", "a"}, "", "Your auth code (optional)")
	addFlag(&runConfig.token, []string{"cookie", "c"}, "", "Your User cookie (optional)")
	addFlag(&runConfig.parallel, []string{"parallel", "p"}, 3, "Parallel task count (default 3)")
	addFlag(&runConfig.blockSize, []string{"block", "b"}, 1200000, "Upload Block Size (default 1200000)")
	addFlag(&runConfig.interval, []string{"timeout", "t"}, 15, "Request retry/timeout limit (in second, default 10)")
	addFlag(&runConfig.prefix, []string{"prefix", "o"}, ".", "File download dictionary/name (default \".\")")
	addFlag(&runConfig.singleMode, []string{"single", "s"}, false, "Single Upload Mode")
	addFlag(&runConfig.debugMode, []string{"verbose", "v"}, false, "Verbose Mode")
	addFlag(&runConfig.keepMode, []string{"keep", "k"}, false, "Keep program active when upload finish")
	addFlag(&runConfig.hashCheck, []string{"hash"}, false, "Check Hash after block upload (might slower)")
	addFlag(&runConfig.passCode, []string{"password"}, "", "Set password")
	addFlag(&runConfig.version, []string{"version"}, false, "Print version and exit")
	addFlag(&runConfig.silentMode, []string{"silent"}, false, "Enable silent mode")
	addFlag(&runConfig.validDays, []string{"valid"}, 1, "Valid Days (default 1)")
	addFlag(&runConfig.shortCode, []string{"short"}, "", "Short Download Code")

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

	if "" != runConfig.shortCode {
		url, err := translate(runConfig.shortCode)
		if err != nil {
			fmt.Printf("unable parse short code: %v\n", err)
			return
		}

		files = append(files, url)
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
		// s := fmt.Sprintf(" %s %s", val[0], val[1])
		block := strings.Repeat(" ", 30-len(val[0]))
		fmt.Printf("%s%s%s\n", val[0], block, val[2])
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
	c := fmt.Sprintf(" --%s", cmd[0])
	if len(cmd) > 1 {
		c += fmt.Sprintf(", -%s", cmd[1])
	}

	s := []string{c, "", usage}
	ptr := unsafe.Pointer(reflect.ValueOf(p).Pointer())
	for _, item := range cmd {
		switch val := val.(type) {
		case int:
			s[1] = "int"
			flag.IntVar((*int)(ptr), item, val, usage)
		case string:
			s[1] = "string"
			flag.StringVar((*string)(ptr), item, val, usage)
		case bool:
			flag.BoolVar((*bool)(ptr), item, val, usage)
		}
	}
	commands = append(commands, s)
}
