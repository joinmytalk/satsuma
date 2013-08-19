package main

import (
	"code.google.com/p/go.net/websocket"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/pat"
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
	apiRouter := pat.New()
	apiRouter.Get("/api/loggedin", http.HandlerFunc(LoggedIn))
	apiRouter.Post("/api/connect", http.HandlerFunc(Connect))
	apiRouter.Post("/api/disconnect", http.HandlerFunc(Disconnect))
	apiRouter.Post("/api/upload", http.HandlerFunc(DoUpload))
	apiRouter.Get("/api/getuploads", http.HandlerFunc(GetUploads))
	apiRouter.Post("/api/delupload", http.HandlerFunc(DeleteUpload))
	apiRouter.Post("/api/startsession", http.HandlerFunc(StartSession))
	apiRouter.Post("/api/stopsession", http.HandlerFunc(StopSession))
	apiRouter.Post("/api/delsession", http.HandlerFunc(DeleteSession))
	apiRouter.Get("/api/getsessions", http.HandlerFunc(GetSessions))
	apiRouter.Get("/api/sessioninfo/{id}", http.HandlerFunc(SessionInfo))
	mux.Handle("/api/ws", websocket.Handler(WebsocketHandler))
	mux.Handle("/api/", apiRouter)

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
