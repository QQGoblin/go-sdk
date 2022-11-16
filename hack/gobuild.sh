#!/bin/bash
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
export GOPROXY=https://goproxy.cn,direct
go build -o helmimg example/helm/main.go

go build -o controller example/controller/main.go