#! /bin/sh
rm -rf dist
mkdir dist
env CGO_ENABLED=0 go build -o dist/picons-update
env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o dist/picons-update-arm
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/picons-update.exe

chmod 0755 dist/picons-update*
