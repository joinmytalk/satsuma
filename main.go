package main

import (
	"code.google.com/p/go.net/websocket"
	"database/sql"
	"github.com/bitly/go-nsq"
	"github.com/bmizerany/pat"
	"github.com/bradrydzewski/go.auth"
	"github.com/fiorix/go-web/autogzip"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"github.com/rcrowley/goagain"
	"github.com/voxelbrain/goptions"
	"net"
	"net/http"
	"os"
	"path"
	"time"
)

const (
	SESSIONNAME     = "SATSUMA_COOKIE"
	XSRFTOKEN       = "XSRF-TOKEN"
	XSRFTOKENHEADER = "X-XSRF-TOKEN"
)

func main() {
	xlog.SetOutput(os.Stdout)

	options := struct {
		Addr                string `goptions:"-L, --listen, description='Listen address'"`
		HashKey             string `goptions:"--hashkey, description='Hash key for cookie store and XSRF', obligatory"`
		BlockKey            string `goptions:"--blockkey, description='Crypto key for cookie store and XSRF', obligatory"`
		GplusClientID       string `goptions:"--gplusclientid, description='Google+ Client ID', obligatory"`
		GplusClientSecret   string `goptions:"--gplusclientsecret, description='Google+ Client Secret', obligatory"`
		GPlusAuthURL        string `goptions:"--gplusauthurl, description='Google+ Authentication URL', obligatory"`
		TwitterClientKey    string `goptions:"--twitterclientkey, description='Twitter Client Key', obligatory"`
		TwitterClientSecret string `goptions:"--twitterclientsecret, description='Twitter Client Secret', obligatory"`
		TwitterAuthURL      string `goptions:"--twitterauthurl, description='Twitter Authentication URL', obligatory"`
		DSN                 string `goptions:"--dsn, description='MySQL DSN string', obligatory"`
		HtdocsDir           string `goptions:"--htdocs, description='htdocs directory', obligatory"`
		UploadDir           string `goptions:"--uploaddir, description='Upload directory', obligatory"`
		TmpDir              string `goptions:"--tmpdir, description='directory for temporary files', obligatory"`
		RedisAddr           string `goptions:"--redis, description='redis address', obligatory"`
		AccessLog           bool   `goptions:"--accesslog, description='log HTTP requests'"`
		StatHat             string `goptions:"--stathat, description='Enable StatHat tracking and set user key'"`
		Topic               string `goptions:"--topic, description='Topic to which uploads shall be published for conversions'"`
		NSQAddr             string `goptions:"--nsqd, description='address:port of nsqd to publish messages to'"`
		PersonaAudience     string `goptions:"--persona-audience, description='Persona audience, e.g. http://localhost:8080'"`
	}{
		Addr:      "[::]:8080",
		RedisAddr: ":6379",
	}
	goptions.ParseAndFail(&options)

	if options.StatHat != "" {
		enableStatHat = true
		stathatUserKey = options.StatHat
	}

	xlog.Debug("Creating cookie store...")
	sessionStore := sessions.NewCookieStore([]byte(options.HashKey), []byte(options.BlockKey))
	secureCookie := securecookie.New([]byte(options.HashKey), []byte(options.BlockKey))

	auth.Config.CookieSecret = []byte(options.HashKey)
	auth.Config.LoginSuccessRedirect = "/api/connect"
	auth.Config.CookieSecure = false

	xlog.Debugf("Connecting to database %s...", options.DSN)

	var dbStore *Store
	if sqldb, err := sql.Open("mysql", options.DSN); err != nil {
		xlog.Fatalf("sql.Open failed: %v", err)
	} else {
		dbStore = NewStore(sqldb)
	}

	fileStore := &FileUploadStore{UploadDir: options.UploadDir, TmpDir: options.TmpDir, Topic: options.Topic, NSQ: nsq.NewWriter(options.NSQAddr)}

	xlog.Debugf("Creating upload directory %s...", options.UploadDir)
	os.Mkdir(options.UploadDir, 0755)

	os.Mkdir(options.TmpDir, 0755)

	xlog.Debugf("Setting up HTTP server...")
	mux := http.NewServeMux()

	// auth calls
	mux.Handle("/auth/gplus", auth.Google(options.GplusClientID, options.GplusClientSecret, options.GPlusAuthURL))
	mux.Handle("/auth/twitter", auth.Twitter(options.TwitterClientKey, options.TwitterClientSecret, options.TwitterAuthURL))
	mux.Handle("/auth/persona", &PersonaAuthHandler{Audience: options.PersonaAudience, SessionStore: sessionStore, DBStore: dbStore, SecureCookie: secureCookie})

	// API calls.
	apiRouter := pat.New()
	apiRouter.Get("/api/loggedin", &LoggedInHandler{SessionStore: sessionStore})
	apiRouter.Get("/api/connect", http.HandlerFunc(auth.SecureUser(func(w http.ResponseWriter, r *http.Request, u auth.User) {
		Connect(w, r, u, sessionStore, secureCookie, dbStore)
	})))
	apiRouter.Get("/api/connected", &ConnectedHandler{SessionStore: sessionStore, DBStore: dbStore})
	apiRouter.Post("/api/disconnect", &DisconnectHandler{SessionStore: sessionStore, SecureCookie: secureCookie})
	apiRouter.Post("/api/upload", &UploadHandler{SessionStore: sessionStore, DBStore: dbStore, UploadStore: fileStore, SecureCookie: secureCookie})
	apiRouter.Get("/api/getuploads", &GetUploadsHandler{SessionStore: sessionStore, DBStore: dbStore})
	apiRouter.Post("/api/renameupload", &RenameUploadHandler{SessionStore: sessionStore, DBStore: dbStore, SecureCookie: secureCookie})
	apiRouter.Post("/api/delupload", &DeleteUploadHandler{SessionStore: sessionStore, DBStore: dbStore, SecureCookie: secureCookie, UploadStore: fileStore})
	apiRouter.Post("/api/startsession", &StartSessionHandler{SessionStore: sessionStore, DBStore: dbStore, SecureCookie: secureCookie})
	apiRouter.Post("/api/stopsession", &StopSessionHandler{SessionStore: sessionStore, DBStore: dbStore, SecureCookie: secureCookie, RedisAddr: options.RedisAddr})
	apiRouter.Post("/api/delsession", &DeleteSessionHandler{SessionStore: sessionStore, DBStore: dbStore, SecureCookie: secureCookie})
	apiRouter.Get("/api/getsessions", &GetSessionsHandler{SessionStore: sessionStore, DBStore: dbStore})
	apiRouter.Get("/api/sessioninfo/:id", &GetSessionInfoHandler{SessionStore: sessionStore, DBStore: dbStore})
	mux.Handle("/api/ws", websocket.Handler(func(c *websocket.Conn) {
		WebsocketHandler(c, dbStore, sessionStore, options.RedisAddr)
	}))
	// let all API things go through autogzip.
	mux.Handle("/api/", autogzip.Handle(apiRouter))

	// deliver index.html through autogzip.
	deliverIndex := autogzip.HandleFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(options.HtdocsDir, "index.html"))
	})

	// deliver index.html for AngularJS routes.
	mux.HandleFunc("/v/", deliverIndex)
	mux.HandleFunc("/s/", deliverIndex)

	// XXX make sure that files from /userdata/ don't go through autogzip. That messes up
	// the load progress of pdf.js.
	mux.Handle("/userdata/", http.StripPrefix("/userdata/", fileStore))

	mux.HandleFunc("/contact", deliverIndex)
	mux.HandleFunc("/tos", deliverIndex)
	mux.HandleFunc("/settings", deliverIndex)

	// deliver static files from htdocs, autogzip'd.
	mux.Handle("/", autogzip.Handle(http.FileServer(http.Dir(options.HtdocsDir))))

	handler := http.Handler(mux)
	if options.AccessLog {
		handler = Logger(handler)
	}

	l, ppid, err := goagain.GetEnvs()
	if err != nil {
		StatCount("satsuma start", 1)
		xlog.Debugf("Starting HTTP server on %s", options.Addr)
		laddr, err := net.ResolveTCPAddr("tcp", options.Addr)
		if err != nil {
			xlog.Fatalf("net.ResolveTCPAddr failed: %v", err)
		}
		l, err = net.ListenTCP("tcp", laddr)
		if err != nil {
			xlog.Fatalf("net.ListenTCP failed: %v", err)
		}
		go http.Serve(l, handler)
	} else {
		StatCount("satsuma reload", 1)
		go http.Serve(l, handler)

		if err := goagain.KillParent(ppid); err != nil {
			xlog.Fatalf("goagain.KillParent failed: %v", err)
		}
	}

	if err := goagain.AwaitSignals(l); nil != err {
		xlog.Fatalf("goagain.AwaitSignals failed: %v", err)
	}

	if err := l.Close(); err != nil {
		xlog.Fatalf("Closing listening socket failed: %v", err)
	}

	// TODO: make sure all requests are finished before exiting, e.g. through a common waitgroup.

	time.Sleep(1 * time.Second)
}
