# cowTransfer-uploader
<a title="Release" target="_blank" href="https://github.com/Mikubill/cowtransfer-uploader/releases"><img src="https://img.shields.io/github/release/Mikubill/cowtransfer-uploader.svg?style=flat-square&hash=c7"></a>
<a title="Go Report Card" target="_blank" href="https://goreportcard.com/report/github.com/Mikubill/cowtransfer-uploader"><img src="https://goreportcard.com/badge/github.com/Mikubill/cowtransfer-uploader?style=flat-square"></a>

Simple cowTransfer Uploader/Downloader

上传/下载文件到奶牛快传的小工具

## usage

Go语言程序, 可直接在[发布页](https://github.com/Mikubill/cowtransfer-uploader/releases)下载使用。

在可执行文件后加上想要上传的文件/想要下载的链接即可食用。

```shell
# upload
./cowtransfer balabala.mp4

# faster upload(?)
./cowtransfer -parallel 12 balabala.mp4

# debug upload
./cowtransfer -verbose balabala.mp4

# download
./cowtransfer https://cowtransfer.com/s/c855d66abd524b
```

## options

```shell
Usage:

  ./cowtransfer [options] file(s)/url(s)

Options:

  -cookie string
    	Your User cookie (optional)
  -parallel int
    	parallel task count (default 4)
  -prefix string
    	file download prefix (default ".")
  -timeout int
    	request retry/timeout limit (default 30)
  -verbose
    	Verbose Mode
```

Note: 

* `-cookie` 可选，可以直接不带任何选项上传文件。
* `-prefix` 指定下载文件的目录。
* `-parallel` 上传并发数，默认为4。如果觉得上传速度太慢也可以试试更高的值。
* `-timeout` 上传超时时间，默认为30秒。
* `-verbose` 开启详细日志，可以看到这个程序每一步都干了啥。

## 缘起

写了acfun-uploader以后有小伙伴让我写一下cowtransfer的，感觉应该也差不多就摸了一个x

(其实完全是小伙伴催出来的qwq，不过奶牛这不按规则的上传处理有点感人2333)