package main

import (
	"code.google.com/p/go.net/websocket"
	"database/sql"
	"github.com/bmizerany/pat"
	"github.com/bradrydzewski/go.auth"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"github.com/voxelbrain/goptions"
	"net/http"
	"os"
	"path"
)

const (
	SESSIONNAME = "SATSUMA_COOKIE"
)

var (
	store   sessions.Store
	options = struct {
		Addr                string `goptions:"-L, --listen, description='Listen address'"`
		CookieKey           string `goptions:"-k, --key, description='Secret key for cookie store', obligatory"`
		GplusClientID       string `goptions:"--gplusclientid, description='Google+ Client ID', obligatory"`
		GplusClientSecret   string `goptions:"--gplusclientsecret, description='Google+ Client Secret', obligatory"`
		TwitterClientKey    string `goptions:"--twitterclientkey, description='Twitter Client Key', obligatory"`
		TwitterClientSecret string `goptions:"--twitterclientsecret, description='Twitter Client Secret', obligatory"`
		DSN                 string `goptions:"--dsn, description='MySQL DSN string', obligatory"`
		HtdocsDir           string `goptions:"--htdocs, description='htdocs directory', obligatory"`
		UploadDir           string `goptions:"--uploaddir, description='Upload directory', obligatory"`
		RedisAddr           string `goptions:"--redis, description='redis address', obligatory"`
	}{
		Addr:      "[::]:8080",
		RedisAddr: ":6379",
	}
)

func main() {
	goptions.ParseAndFail(&options)

	xlog.Debug("Creating cookie store...")
	store = sessions.NewCookieStore([]byte(options.CookieKey))

	auth.Config.CookieSecret = []byte(options.CookieKey)
	auth.Config.LoginSuccessRedirect = "/api/connect"
	auth.Config.CookieSecure = false

	xlog.Debugf("Connecting to database %s...", options.DSN)

	var dbStore *Store
	if sqldb, err := sql.Open("mysql", options.DSN); err != nil {
		xlog.Fatalf("sql.Open failed: %v", err)
	} else {
		dbStore = NewStore(sqldb)
	}

	xlog.Debugf("Creating upload directory %s...", options.UploadDir)
	os.Mkdir(options.UploadDir, 0755)

	xlog.Debugf("Setting up HTTP server...")
	mux := http.NewServeMux()

	// auth calls
	mux.Handle("/auth/gplus", auth.Google(options.GplusClientID, options.GplusClientSecret, "http://localhost:8080/auth/gplus"))
	mux.Handle("/auth/twitter", auth.Twitter(options.TwitterClientKey, options.TwitterClientSecret, "http://localhost:8080/auth/twitter"))

	// API calls.
	apiRouter := pat.New()
	apiRouter.Get("/api/loggedin", http.HandlerFunc(LoggedIn))
	apiRouter.Get("/api/connect", http.HandlerFunc(auth.SecureUser(Connect)))
	apiRouter.Post("/api/disconnect", http.HandlerFunc(Disconnect))
	apiRouter.Post("/api/upload", &UploadHandler{DBStore: dbStore})
	apiRouter.Get("/api/getuploads", &GetUploadsHandler{DBStore: dbStore})
	apiRouter.Post("/api/renameupload", &RenameUploadHandler{DBStore: dbStore})
	apiRouter.Post("/api/delupload", &DeleteUploadHandler{DBStore: dbStore})
	apiRouter.Post("/api/startsession", &StartSessionHandler{DBStore: dbStore})
	apiRouter.Post("/api/stopsession", &StopSessionHandler{DBStore: dbStore})
	apiRouter.Post("/api/delsession", &DeleteSessionHandler{DBStore: dbStore})
	apiRouter.Get("/api/getsessions", &GetSessionsHandler{DBStore: dbStore})
	apiRouter.Get("/api/sessioninfo/:id", &GetSessionInfoHandler{DBStore: dbStore})
	mux.Handle("/api/ws", websocket.Handler(func(c *websocket.Conn) {
		WebsocketHandler(c, dbStore)
	}))
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
