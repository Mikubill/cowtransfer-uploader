#!/usr/bin/env bash

set -e

hash tar uname grep curl head
OS="$(uname)"
case $OS in
  Linux)
    OS='linux'
    ;;
  FreeBSD)
    OS='freebsd'
    ;;
  Darwin)
    OS='darwin'
    ;;
  *)
    echo 'OS not supported'
    exit 2
    ;;
esac

ARCH="$(uname -m)"
case $ARCH in
  x86_64|amd64)
    ARCH='amd64'
    ;;
  aarch64)
    ARCH='arm64'
    ;;
  *)
    echo 'OS type not supported'
    exit 2
    ;;
esac

VERSION=$(curl https://api.github.com/repos/Mikubill/cowtransfer-uploader/releases/latest 2>&1 | grep -Po '[0-9]+\.[0-9]+\.[0-9]+' | head -1)

curl -L https://github.com/Mikubill/cowtransfer-uploader/releases/download/v$VERSION/cowtransfer-uploader_$VERSION\_$OS\_$ARCH.tar.gz | tar xz

printf "\nCowTransfer-uploader Downloded.\n\n"
exit 0
