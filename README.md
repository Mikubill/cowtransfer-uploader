# cowTransfer-uploader

Simple cowTransfer Uploader

上传文件到奶牛快传的小工具

## usage

Go语言程序, 可直接在[发布页](https://github.com/Mikubill/acfun-uploader/releases)下载使用。


```shell
./cowtransfer-uploader [options] file(s)

  -cookie string
        Your User cookie (optional)
  -parallel int
        parallel task count (default 4)
  -verbose
        Verbose Mode

```

Note: cookie可选，可以直接不带任何选项上传文件；parallel控制的是并发数量；verbose开启详细日志，可以看到这个程序每一步都干了啥

## 缘起

写了acfun-uploader以后有小伙伴让我写一下cowtransfer的，感觉应该也差不多就摸了一个x

(其实完全是小伙伴催出来的qwq，不过奶牛这不按规则的上传处理有点感人2333)