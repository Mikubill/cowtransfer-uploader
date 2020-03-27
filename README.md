# cowTransfer-uploader

Simple cowTransfer Uploader

上传文件到奶牛快传的小工具

## usage

Go语言程序, 可直接在[发布页](https://github.com/Mikubill/cowtransfer-uploader/releases)下载使用。


```shell
./cowtransfer-uploader [options] file(s)

  -cookie string
        Your User cookie (optional)
  -parallel int
        parallel task count (default 4) (default 4)
  -timeout int
        request retry/timeout limit (default 30s) (default 30)
  -verbose
        Verbose Mode
```

Note: 

* `-cookie` 可选，可以直接不带任何选项上传文件。
* `-parallel` 上传并发数，默认为4。如果觉得上传速度太慢也可以试试更高的值。
* `-timeout` 上传超时时间，默认为30秒。
* `-verbose` 开启详细日志，可以看到这个程序每一步都干了啥。

## 缘起

写了acfun-uploader以后有小伙伴让我写一下cowtransfer的，感觉应该也差不多就摸了一个x

(其实完全是小伙伴催出来的qwq，不过奶牛这不按规则的上传处理有点感人2333)