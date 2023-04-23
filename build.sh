#! /bin/sh
rm -rf dist
mkdir dist
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/picons-update-amd64-linux
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/picons-update-amd64-darwin
env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o dist/picons-update-arm
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/picons-update.exe

chmod 0755 dist/picons-update*
