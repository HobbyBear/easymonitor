#!/bin/sh

cd webhookserver
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build
mv -f webhookserver ../program

cd ../webapp
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build
mv -f  webapp ../program




