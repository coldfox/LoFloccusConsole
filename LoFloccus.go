/*
LoFloccus - Sync Floccus to a Local Folder!
*/

package main

import (
	"context"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"golang.org/x/net/webdav"
	"gopkg.in/ini.v1"

	"log"
	"net/http"
	"strconv"
	"strings"
)

type ServerConfig struct {
	address string
	port    int
	dir     string
	user    string
	passwd  string
}

var (
	appVersion   = "1.1.2"
	configFile   = "./LoFloccus-Settings.ini"
	cfg          *ini.File
	serverHandle *http.Server
	serverConfig = ServerConfig{
		address: "127.0.0.1",
		port:    0,
		dir:     "",
		user:    "floccus",
		passwd:  "",
	}
)

func main() {

	// Load AppConfig from config file
	loadAppConfig()
	serverStart()
}
func exitApp() {
	serverStop()
}

/*获取当前文件执行的路径*/
func GetCurPath() string {
	file, _ := exec.LookPath(os.Args[0])

	//得到全路径，比如在windows下E:\\golang\\test\\a.exe
	path, _ := filepath.Abs(file)

	rst := filepath.Dir(path)

	return rst
}

func loadAppConfig() {
	cfgPath := path.Join(GetCurPath(), configFile)
	var err error
	cfg, err = ini.Load(cfgPath)
	if err != nil {
		log.Printf("No config file %v found. Creating a new one from defaults.", err)
		cfg = ini.Empty()

		// Generate random defaults
		rand.Seed(time.Now().UnixNano())
		serverConfig.passwd = "local-" + strconv.Itoa(rand.Intn(999-100)+100)
		serverConfig.port = rand.Intn(65535-40000) + 40000

		// Save App Config
		saveAppConfig()
	} else {
		// ServerConfig Section
		serverConfig.address = cfg.Section("ServerConfig").Key("address").String()
		serverConfig.port, err = cfg.Section("ServerConfig").Key("port").Int()
		serverConfig.dir = cfg.Section("ServerConfig").Key("dir").String()
		serverConfig.user = cfg.Section("ServerConfig").Key("user").String()
		serverConfig.passwd = cfg.Section("ServerConfig").Key("passwd").String()
	}

	log.Printf("server listen port:%v.", serverConfig.port)
}

func saveAppConfig() {
	// ServerConfig Section
	cfg.Section("ServerConfig").Key("address").SetValue(serverConfig.address)
	cfg.Section("ServerConfig").Key("port").SetValue(strconv.Itoa(serverConfig.port))
	cfg.Section("ServerConfig").Key("dir").SetValue(serverConfig.dir)
	cfg.Section("ServerConfig").Key("passwd").SetValue(serverConfig.passwd)
	cfg.Section("ServerConfig").Key("user").SetValue(serverConfig.user)

	// Save new settings
	cfg.SaveTo(configFile)
}

func serverStart() {

	log.Printf("WEBDAV: Starting...")

	bookMarksdir := path.Join(GetCurPath(), serverConfig.dir)
	dir := webdav.Dir(bookMarksdir)
	mux := http.NewServeMux()
	srvWebdav := &webdav.Handler{
		FileSystem: dir,
		LockSystem: webdav.NewMemLS(),
		Logger: func(request *http.Request, err error) {
			if err != nil {
				log.Printf("WEBDAV [%s]: %s, ERROR: %s\n", request.Method, request.URL, err)
			} else {
				log.Printf("WEBDAV [%s]: %s \n", request.Method, request.URL)
			}
		},
	}

	serverHandle = &http.Server{
		Addr:    serverConfig.address + ":" + strconv.Itoa(serverConfig.port),
		Handler: mux,
	}

	mux.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {

		// Authentication
		username, password, ok := request.BasicAuth()
		if !ok {
			response.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		if username != serverConfig.user || password != serverConfig.passwd {
			http.Error(response, "WebDAV: need authorized!", http.StatusUnauthorized)
			return
		}

		//支持customdav方式的数据
		valid := false
		if strings.Contains(request.RequestURI, "customdav") {
			valid = true
		}

		// Restrict WebDav to the current folder & read/writes to .xbel files
		if (!valid && !strings.HasSuffix(request.RequestURI, ".xbel") && !strings.HasSuffix(request.RequestURI, ".xbel.lock") && request.RequestURI != "/") || (request.RequestURI == "/" && (request.Method != "HEAD" && request.Method != "PROPFIND")) {
			errorFsAccessMsg := "LoFloccus: unauthorized filesystem access detected. LoFloccus WebDAV server is restricted to '*.xbel' files."
			log.Printf(request.RequestURI)
			log.Printf(request.Method)
			log.Printf(errorFsAccessMsg)
			http.Error(response, errorFsAccessMsg, http.StatusUnauthorized)
			return
		}

		srvWebdav.ServeHTTP(response, request)
	})
	if err := serverHandle.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Error with WebDAV server: %s", err)
	}

}

func serverStop() {
	log.Printf("WEBDAV: Shutting down...")
	if err := serverHandle.Shutdown(context.TODO()); err != nil {
		log.Fatalf("WEBDAV: error shutting down - %s", err)
	}
	serverHandle = nil
	log.Printf("WEBDAV: Server is down.")
}
