#!/bin/bash
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
export GOPROXY=https://goproxy.cn
go build -o helmimg cmd/helm/images.go