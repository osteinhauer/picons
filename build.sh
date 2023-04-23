#! /bin/bash
version="$(git symbolic-ref -q --short HEAD || git describe --tags --exact-match)"
gitsha1="$(git rev-parse HEAD)"
version="${version/main/develop-${gitsha1}}"
version="${version/release\//}"

echo "build $version"

rm -rf dist
mkdir dist
echo build linux
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/picons-update-amd64-linux -v -ldflags="-X 'main.version=$version'"
echo build macos
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/picons-update-amd64-darwin -v -ldflags="-X 'main.version=$version'"
echo build enigma2 arm
env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -o dist/picons-update-arm -v -ldflags="-X 'main.version=$version'"
echo build windows
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/picons-update.exe -v -ldflags="-X 'main.version=$version'"

chmod 0755 dist/picons-update*
