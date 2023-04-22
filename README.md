# Picons

## run

```
picons-update --help

Usage:
  picons-update [OPTIONS]

Application Options:
  -f, --picons-folder=                  picons Verzeichnis auf vu (default: /usr/share/enigma2/picon/)
  -c, --copy                            ob die picons kopiert werden müssen
  -t, --temp-dir=                       temp Verzeichnis vor dem kopieren zu remote
  -k, --pem-file=                       pemfile für ssh
  -h, --host=                           host zum kopieren (default: vuuno4kse)
  -p, --pushover-token=                 pushover token
  -r, --pushover-recipient=             pushover recipient
  -P, --pushover-priority=[-2|-1|0|1|2] pushover prio (default: 0)
      --dry-run                         nur prüfen
  -u, --picons-remote-folder=           Pfad der picons auf dem Server (default: picons/uploader/NaseDC/by Name_13.0&19.2E_DVB-C_T2_NaseDC_XPicons_transparent_220x132_32
                                        Bit_NaseDC)
  -l, --load-by=[name|ref]              ob die picons auf dem Server by name oder by ref sind (default: name)
  -s, --save-as=[name|ref|all]          ob die picons auf der vu by name, by ref oder beides gespeichert werden sollen (default: all)
  -L, --lastrun                         prüft anhand einer .picons-update.lastrun im picons folder, ob update ausgeführt wird
  -I, --info                            info vom remote server

Help Options:
  -h, --help                            Show this help message
```

## build

```
./build.sh
```

```
LOG_LEVEL=debug go run main.go
```

```
go build
env GOOS=linux GOARCH=arm go build -o picons-arm
```
