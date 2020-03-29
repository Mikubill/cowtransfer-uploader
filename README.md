# cowTransfer-uploader
<a title="Release" target="_blank" href="https://github.com/Mikubill/cowtransfer-uploader/releases"><img src="https://img.shields.io/github/release/Mikubill/cowtransfer-uploader.svg?style=flat-square&hash=c7"></a>
<a title="Go Report Card" target="_blank" href="https://goreportcard.com/report/github.com/Mikubill/cowtransfer-uploader"><img src="https://goreportcard.com/badge/github.com/Mikubill/cowtransfer-uploader?style=flat-square"></a>

Simple cowTransfer Uploader/Downloader

上传/下载文件到奶牛快传的小工具，支持分块并发上传下载

## install

Go语言程序, 可直接在[发布页](https://github.com/Mikubill/cowtransfer-uploader/releases)下载使用。

linux的小伙伴也可以使用这个命令来下载：

```shell
curl -sL https://git.io/cowtransfer | sh 
```

## usage

在cowtransfer-uploader后加上想要上传的文件/文件夹即可上传，可以手动修改parallels来提高上传并发数。

```shell
# upload
./cowtransfer-uploader balabala.mp4

# upload folder
./cowtransfer-uploader /usr

# faster upload(?)
./cowtransfer-uploader -p 12 balabala.mp4
```

程序默认会为每一个文件生成一个链接。如果想一个链接上传所有文件，可以使用选项`-s`开启Single Upload Mode：

```shell
# single upload mode
./cowtransfer-uploader -s /usr
```

在cowtransfer-uploader后加上文件分享链接即可下载，可以手动修改parallels来提高下载并发数。


```shell
# download
./cowtransfer-uploader https://c-t.work/s/c855d66abd524b

# faster download(?)
./cowtransfer-uploader -p 8 https://c-t.work/s/c855d66abd524b
```


## options

```shell

Usage:

  ./cowtransfer-uploader [options] file(s)/url(s)

Options:

  -c, --cookie string         Your User cookie (optional)
  -p, --parallel int          Parallel task count (default 4)
  -t, --timeout int           Request retry/timeout limit (in second, default 30)
  -o, --output string         File download dictionary/name (default ".")
  -s, --single                Single Upload Mode
  -v, --verbose               Verbose Mode
  -k, --keep                  Keep program active when upload finish
  --version                   Print version and exit

```

Note: 

* `-c, --cookie` 可选，可以直接不带任何选项上传文件。
* `-o, --output` 指定下载文件的目录。（也可以使用`-prefix`指定）
* `-p, --parallel` 上传/下载并发数，默认为4。如果觉得速度太慢也可以试试更高的值。
* `-t, --timeout` 上传超时时间，默认为30秒。
* `-v, --verbose` 开启详细日志，可以看到这个程序每一步都干了啥。
* `-k, --keep` 在上传完毕后不立即退出，在某些情况下可能有用。
* `--version` 显示当前版本号。

## 常见问题

1. 进度条卡住了/速度太慢/速度为零

七牛云上传接口/cowtransfer API均有可能不稳定，如确认网络正常可使用较低的超时时间重试，即：

```shell 
./cowtransfer-uploader -t 3 file
```

## 缘起

写了acfun-uploader以后有小伙伴让我写一下cowtransfer的，感觉应该也差不多就摸了一个x

(其实完全是小伙伴催出来的qwq，不过奶牛这不按规则的上传处理有点感人2333)
