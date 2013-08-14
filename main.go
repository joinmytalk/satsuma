package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"github.com/voxelbrain/goptions"
	"net/http"
	"os"
	"path"
)

const (
	SESSION_NAME = "SATSUMA_COOKIE"
)

type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IdToken     string `json:"id_token"`
}

// ClaimSet represents an IdToken response.
type ClaimSet struct {
	Sub string
}

var (
	store   sessions.Store
	sqlDB   *sql.DB
	options = struct {
		Addr         string `goptions:"-L, --listen, description='Listen address'"`
		CookieKey    string `goptions:"-k, --key, description='Secret key for cookie store', obligatory"`
		ClientID     string `goptions:"--clientid, description='Client ID', obligatory"`
		ClientSecret string `goptions:"--clientsecret, description='Client Secret', obligatory"`
		DSN          string `goptions:"--dsn, description='MySQL DSN string', obligatory"`
		HtdocsDir    string `goptions:"--htdocs, description='htdocs directory', obligatory"`
		UploadDir    string `goptions:"--uploaddir, description='Upload directory', obligatory"`
	}{
		Addr: "[::]:8080",
	}
)

func main() {
	goptions.ParseAndFail(&options)

	xlog.Debug("Creating cookie store...")
	store = sessions.NewCookieStore([]byte(options.CookieKey))

	xlog.Debugf("Connecting to database %s...", options.DSN)
	if sqldb, err := sql.Open("mysql", options.DSN); err != nil {
		xlog.Fatalf("sql.Open failed: %v", err)
	} else {
		sqlDB = sqldb
	}

	xlog.Debugf("Creating upload directory %s...", options.UploadDir)
	os.Mkdir(options.UploadDir, 0755)

	xlog.Debugf("Setting up HTTP server...")
	mux := http.NewServeMux()

	// API calls.
	mux.HandleFunc("/api/loggedin", LoggedIn)
	mux.HandleFunc("/api/connect", Connect)
	mux.HandleFunc("/api/disconnect", Disconnect)
	mux.HandleFunc("/api/upload", DoUpload)
	mux.HandleFunc("/api/getuploads", GetUploads)
	mux.HandleFunc("/api/startsession", StartSession)
	mux.HandleFunc("/api/getsessions", GetSessions)

	// deliver index.html for AngularJS routes.
	mux.HandleFunc("/v/", DeliverIndex)
	mux.HandleFunc("/s/", DeliverIndex)

	// deliver static files from htdocs.
	mux.Handle("/", http.FileServer(http.Dir(options.HtdocsDir)))

	xlog.Debugf("Starting HTTP server on %s", options.Addr)
	httpsrv := &http.Server{Handler: Logger(mux), Addr: options.Addr}
	if err := httpsrv.ListenAndServe(); err != nil {
		xlog.Fatalf("ListenAndServe: %v", err)
	}
}

func DeliverIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, path.Join(options.HtdocsDir, "index.html"))
}
