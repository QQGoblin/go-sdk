#!/bin/bash
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
export GOPROXY=https://goproxy.cn,direct

go build -o bin/certs               example/certs/main.go
go build -o bin/leader_by_etcd      example/leader/etcd/main.go
go build -o bin/leader_by_apiserver example/leader/apiserver/main.go