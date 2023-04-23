package main

import (
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/briandowns/spinner"
	"github.com/go-resty/resty/v2"
	"github.com/gregdel/pushover"
	"github.com/jessevdk/go-flags"
	"github.com/jwalton/go-supportscolor"
	"github.com/povsister/scp"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

const sRefTV = "1:7:1:0:0:0:0:0:0:0:(type+==+1)+||+(type+==+17)+||+(type+==+22)+||+(type+==+24)+||+(type+==+25)+||+(type+==+27)+||+(type+==+31)+ORDER+BY+name"
const sRefRadio = "1:7:2:0:0:0:0:0:0:0:(type+==+2)+||+(type+==+10)+ORDER+BY+name"

var piconsBaseUrl = "http://picons.vuplus-support.org/"

var piconsFolderIsRemote bool

var client = resty.New()

var opts Options

var version = ""

type Options struct {
	PiconsFolder       string `short:"f" long:"picons-folder" default:"/usr/share/enigma2/picon/" description:"picons Verzeichnis auf vu"`
	Copy               bool   `short:"c" long:"copy" description:"ob die picons kopiert werden müssen"`
	Tempdir            string `short:"t" long:"temp-dir" description:"temp Verzeichnis vor dem kopieren zu remote"`
	Pemfile            string `short:"k" long:"pem-file" description:"pemfile für ssh"`
	Host               string `short:"h" long:"host" default:"vuuno4kse" description:"host zum kopieren"`
	PushoverToken      string `short:"p" long:"pushover-token" description:"pushover token"`
	PushoverRecipient  string `short:"r" long:"pushover-recipient" description:"pushover recipient"`
	PushoverPriority   int    `short:"P" long:"pushover-priority" description:"pushover prio" default:"0" choice:"-2" choice:"-1" choice:"0" choice:"1" choice:"2"`
	DryRun             bool   `long:"dry-run" description:"nur prüfen"`
	PiconsRemoteFolder string `short:"u" long:"picons-remote-folder" description:"Pfad der picons auf dem Server" default:"picons/uploader/NaseDC/by Name_13.0&19.2E_DVB-C_T2_NaseDC_XPicons_transparent_220x132_32 Bit_NaseDC"`
	LoadBy             string `short:"l" long:"load-by" description:"ob die picons auf dem Server by name oder by ref sind" default:"name" choice:"name" choice:"ref"`
	SaveAs             string `short:"s" long:"save-as" description:"ob die picons auf der vu by name, by ref oder beides gespeichert werden sollen" default:"all" choice:"name" choice:"ref" choice:"all"`
	Lastrun            bool   `short:"L" long:"lastrun" description:"prüft anhand einer .picons-update.lastrun im picons folder, ob update ausgeführt wird"`
	Info               bool   `short:"I" long:"info" description:"info vom remote server"`
	Version            bool   `short:"v" long:"version" description:"programm version"`
}

type LoadResult struct {
	foundRefs   []Ref
	missingRefs []Ref
	skipedRefs  []Ref
}

func (loadResult LoadResult) missingRefsCount() int {
	return len(loadResult.missingRefs)
}

func (loadResult LoadResult) skipedRefsCount() int {
	return len(loadResult.skipedRefs)
}

func (loadResult LoadResult) foundRefsCount() int {
	return len(loadResult.foundRefs)
}

type Ref struct {
	Servicereference string `json:"servicereference"`
	Startpos         int    `json:"startpos"`
	Program          int    `json:"program"`
	Servicename      string `json:"servicename"`
	Pos              int    `json:"pos"`
}

type Response struct {
	Services []Ref `json:"services"`
	Pos      int   `json:"pos"`
}

func (ref Ref) filenameByOptions() string {
	switch opts.LoadBy {
	case "ref":
		return ref.filenameByRef()
	default:
		return ref.filenameByName()
	}
}

func (ref Ref) filenamesByOptions() []string {
	switch opts.SaveAs {
	case "ref":
		return []string{ref.filenameByRef()}
	case "name":
		return []string{ref.filenameByName()}
	default:
		return []string{ref.filenameByRef(), ref.filenameByName()}
	}
}

func (ref Ref) filenameByName() string {
	return ref.Servicename + ".png"
}

func (ref Ref) filenameByRef() string {
	return strings.TrimSuffix(strings.ReplaceAll(ref.Servicereference, ":", "_"), "_") + ".png"
}

func (ref Ref) isDotOnlyName() bool {
	return strings.TrimSpace(strings.ReplaceAll(ref.Servicename, ".", "")) == ""
}

func (ref Ref) isSkipableName() bool {
	return ref.isDotOnlyName() || strings.Contains(ref.Servicename, "/")
}

func (ref Ref) filenameByNameNormalized() string {
	normalized := strings.ToLower(cleanWhitespaces(ref.Servicename))
	normalized = strings.ReplaceAll(normalized, "*", "star")
	normalized = strings.ReplaceAll(normalized, "+", "plus")
	normalized = strings.ReplaceAll(normalized, "&", "and")
	normalized = strings.ReplaceAll(normalized, "ä", "ae")
	normalized = strings.ReplaceAll(normalized, "ü", "ue")
	normalized = strings.ReplaceAll(normalized, "ö", "oe")
	normalized = strings.ReplaceAll(normalized, "ß", "sst")

	if utf8.RuneCountInString(normalized) > 2 {
		normalized = strings.TrimSuffix(normalized, "hd")
	}

	return strings.TrimSpace(normalized) + ".png"
}

type PiconInfo struct {
	Path        string
	Example     string
	Date        string
	Description string
	Type        string
	Author      string
	Color       string
	Resolution  string
	Uploader    string
}

func getInfo() PiconInfo {
	resp, err := client.R().EnableTrace().Get("http://picons.vuplus-support.org/picons/picon_info.txt")
	if err != nil || resp.IsError() {
		log.Error("Fehler beim Abrufen der Infos: ", err)
		os.Exit(1)
	}
	r := csv.NewReader(strings.NewReader(resp.String()))
	r.Comma = ';'
	r.FieldsPerRecord = 9
	r.LazyQuotes = true
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		if record[0] == opts.PiconsRemoteFolder {
			return PiconInfo{
				Path:        record[0],
				Example:     record[1],
				Date:        record[2],
				Description: record[3],
				Type:        record[4],
				Author:      record[5],
				Color:       record[6],
				Resolution:  record[7],
				Uploader:    record[8]}
		}
	}

	return PiconInfo{}
}

func getServices(sRef string) Response {
	resp, err := client.R().
		EnableTrace().
		Get("http://" + opts.Host + "/api/getservices?sRef=" + sRef)

	log.Debug("URL: ", resp.Request.URL)
	log.Debug("Body: ", resp)

	if err != nil || resp.IsError() {
		log.Error("Fehler beim Abrufen der Services: ", err)
		os.Exit(1)
	}

	var response Response
	json.Unmarshal(resp.Body(), &response)

	log.Debug("Response: ", response)

	return response
}

func getPicon(name string) []byte {

	var piconsUrl = piconsBaseUrl + "/" + name

	log.Debug("picon URL: ", piconsBaseUrl)

	resp, err := client.R().
		EnableTrace().
		Get(piconsUrl)

	if err != nil || resp.IsError() {
		return nil
	}

	return resp.Body()
}

func cleanWhitespaces(s string) string {
	space := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(space.ReplaceAllString(s, " "))
}

func quote(s string) string {
	return "\"" + s + "\""
}

func toCSV(service Ref, kind string) string {
	return quote(service.Servicename) + "," +
		quote(kind) + "," +
		quote(service.filenameByName()) + "," +
		quote(service.filenameByRef())
}

func forceGetPicon(service Ref, name string) ([]byte, bool) {
	picon := getPicon(name)
	if picon != nil {
		log.Debug("gefunden: ", name)
		return picon, true
	} else {
		forceName := cleanWhitespaces(name)
		log.Debug("versuche: ", forceName)
		picon := getPicon(forceName)
		if picon != nil {
			log.Debug("gefunden: ", forceName)
			return picon, false
		} else {
			log.Debug("nicht gefunden: ", forceName)
			return nil, false
		}
	}
}

func saveFile(picon []byte, file string) {
	err := ioutil.WriteFile(opts.Tempdir+"/"+file, picon, 0644)
	if err != nil {
		log.Error("Error: ", err)
	}
}

func savePicon(picon []byte, service Ref) {
	if !opts.DryRun {
		for _, filename := range service.filenamesByOptions() {
			saveFile(picon, filename)
		}
	}
}

func load(refs []Ref, onNext func(ref Ref)) LoadResult {

	var foundRefs []Ref
	var missingRefs []Ref
	var skipedRefs []Ref

	for _, service := range refs {
		onNext(service)
		if service.isSkipableName() {
			log.Debug("skip ", service)
			skipedRefs = append(skipedRefs, service)
			continue
		}
		log.Info("Suche für ", service.Servicename)

		filename := service.filenameByName()
		picon, found := forceGetPicon(service, filename)
		if picon != nil {
			log.Debug("gefunden: ", filename)
			savePicon(picon, service)
		}
		if found {
			foundRefs = append(foundRefs, service)
		} else {
			missingRefs = append(missingRefs, service)
		}
	}

	return LoadResult{
		foundRefs:   foundRefs,
		missingRefs: missingRefs,
		skipedRefs:  skipedRefs}
}

func copyToRemote(host string) {
	privPEM, _ := ioutil.ReadFile(opts.Pemfile)
	// without passphrase
	sshConf, _ := scp.NewSSHConfigFromPrivateKey("root", privPEM)
	scpClient, err := scp.NewClient(host, sshConf, &scp.ClientOption{})

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer scpClient.Close()
	scpClient.CopyDirToRemote(opts.Tempdir+"/.", opts.PiconsFolder, &scp.DirTransferOption{})
}

func pathNotExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true
	}

	return false
}

func print(refsTV []Ref, refsRadio []Ref, pretext string) {
	if len(refsTV) > 0 || len(refsRadio) > 0 {
		fmt.Print(pretext)
		fmt.Println(quote("channel") + "," + quote("type") + "," + quote("by name") + "," + quote("by ref"))
		for _, service := range refsTV {
			fmt.Println(toCSV(service, "TV"))
		}
		for _, service := range refsRadio {
			fmt.Println(toCSV(service, "Radio"))
		}
	}
}

func pushToPushover(text, title string) {
	fmt.Println("sende an pushover")
	app := pushover.New(opts.PushoverToken)
	recipient := pushover.NewRecipient(opts.PushoverRecipient)

	message := &pushover.Message{
		Message:  text,
		Title:    title,
		Priority: opts.PushoverPriority,
	}

	response, err := app.SendMessage(message, recipient)
	if err != nil {
		log.Error(err)
	}
	log.Debug(response)
}

func lastrunFileIsSame(piconInfo PiconInfo, refs []Ref) bool {
	data, err := ioutil.ReadFile(opts.PiconsFolder + "/.picons-update.lastrun")
	if err != nil {
		return false
	}
	checksum := checksum(refs)
	lastrun := strings.TrimSpace(string(data))
	return lastrun == piconInfo.Date+" "+checksum
}

func checksum(refs []Ref) string {
	refsJson, _ := json.Marshal(refs)
	hash := md5.Sum(refsJson)
	return hex.EncodeToString(hash[:])
}

func writeLastrun(piconInfo PiconInfo, refs []Ref) {
	var folder = opts.PiconsFolder
	if piconsFolderIsRemote {
		folder = opts.Tempdir
	}
	content := piconInfo.Date + " " + checksum(refs)
	ioutil.WriteFile(folder+"/.picons-update.lastrun", []byte(content), 0644)
}

func initOptions() {

	_, err := flags.Parse(&opts)

	if err != nil {
		os.Exit(0)
	}

	piconsBaseUrl = piconsBaseUrl + opts.PiconsRemoteFolder

	if opts.Copy {
		piconsFolderIsRemote = true
	} else {
		piconsFolderIsRemote = pathNotExist(opts.PiconsFolder)
	}

	if !opts.DryRun {
		if piconsFolderIsRemote {
			var dir string

			if opts.Tempdir != "" {
				dir = opts.Tempdir
			} else {
				dir, err = ioutil.TempDir("", "picons_")
				log.Debug("Temp Dir: ", dir)
				if err != nil {
					log.Fatal(err)
					os.Exit(1)
				}
			}
			opts.Tempdir = dir
			os.Mkdir(dir, 0755)
		} else {
			opts.Tempdir = opts.PiconsFolder
		}

		if opts.Pemfile == "" {
			userhome, _ := os.UserHomeDir()
			opts.Pemfile = userhome + "/.ssh/id_rsa"
		}
	}
}

func init() {
	log.SetOutput(os.Stdout)
	// log.SetFormatter(&log.JSONFormatter{})

	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.ErrorLevel
	}

	log.SetLevel(logLevel)

	initOptions()
}

func main() {
	if opts.Version {
		fmt.Println("picons-update version: " + version)
		os.Exit(0)
	}

	if piconsFolderIsRemote {
		defer os.RemoveAll(opts.Tempdir)
	}

	piconInfo := getInfo()

	fmt.Printf("picons '%s' vom %s\n", piconInfo.Description, piconInfo.Date)

	if opts.Info {
		os.Exit(0)
	}

	var refsTV = getServices(sRefTV).Services
	var refsRadio = getServices(sRefRadio).Services

	if opts.Lastrun && !opts.DryRun && !piconsFolderIsRemote {
		if lastrunFileIsSame(piconInfo, append(refsTV, refsRadio...)) {
			fmt.Printf("%s: picons von %s bereits geladen\n", time.Now().Format("02.01.2006 15:04"), piconInfo.Date)
			os.Exit(0)
		}
	}

	piconsSum := len(refsTV) + len(refsRadio)
	supportscolor := supportscolor.Stdout().SupportsColor

	onNext := func(ref Ref) {}
	if supportscolor {
		bar := progressbar.Default(int64(piconsSum))
		onNext = func(ref Ref) {
			bar.Add(1)
		}
	} else {
		fmt.Printf("lade %d picons\n", piconsSum)
	}

	loadResultTV := load(refsTV, onNext)
	loadResultRadio := load(refsRadio, onNext)

	foundCount := loadResultTV.foundRefsCount() + loadResultRadio.foundRefsCount()
	missingCount := loadResultTV.missingRefsCount() + loadResultRadio.missingRefsCount()
	skipedCount := loadResultTV.skipedRefsCount() + loadResultRadio.skipedRefsCount()

	if opts.DryRun {
		fmt.Printf("\n%d picons geladen, %d nicht gefunden und %d übersprungen\n", foundCount, missingCount, skipedCount)
	} else {
		fmt.Printf("\n%d picons geladen, %d nicht gefunden und %d übersprungen: %v\n", foundCount, missingCount, skipedCount, opts.Tempdir)
	}

	print(loadResultTV.missingRefs, loadResultRadio.missingRefs, "\nFehlende picons:\n\n")
	print(loadResultTV.skipedRefs, loadResultRadio.skipedRefs, "\nÜbersprungene picons:\n\n")

	if !opts.DryRun {

		writeLastrun(piconInfo, append(refsTV, refsRadio...))

		if piconsFolderIsRemote {
			if supportscolor {
				s := spinner.New(spinner.CharSets[17], 200*time.Millisecond)
				fmt.Println("")
				s.Suffix = " Upload " + opts.Host + " " + opts.PiconsFolder
				s.Start()
				copyToRemote(opts.Host)
				s.Stop()
				fmt.Println()
			} else {
				fmt.Printf(" Upload %s %s\n", opts.Host, opts.PiconsFolder)
				copyToRemote(opts.Host)
			}
		}

		if missingCount > 0 {
			if opts.PushoverToken != "" && opts.PushoverRecipient != "" {
				pushToPushover(strconv.Itoa(missingCount)+" picons fehlen", "picons")
			}
		}
	}
}
