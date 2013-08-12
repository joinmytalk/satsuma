package main

import (
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"github.com/voxelbrain/goptions"
	"labix.org/v2/mgo"
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
	mongoDB *mgo.Database
	options = struct {
		Addr         string `goptions:"-L, --listen, description='Listen address'"`
		CookieKey    string `goptions:"-k, --key, description='Secret key for cookie store', obligatory"`
		ClientID     string `goptions:"--clientid, description='Client ID', obligatory"`
		ClientSecret string `goptions:"--clientsecret, description='Client Secret', obligatory"`
		MongoURL     string `goptions:"--mongodb, description='MongoDB connect string', obligatory"`
		HtdocsDir    string `goptions:"--htdocs, description='htdocs directory', obligatory"`
		UploadDir    string `goptions:"--uploaddir, description='Upload directory', obligatory"`
	}{
		Addr: "[::]:8080",
	}
)

func main() {
	goptions.ParseAndFail(&options)

	store = sessions.NewCookieStore([]byte(options.CookieKey))

	mongoSession, err := mgo.Dial(options.MongoURL)
	if err != nil {
		xlog.Fatalf("mgo.Dial failed: %v", err)
	}
	mongoDB = mongoSession.DB("")

	os.Mkdir(options.UploadDir, 0755)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/logged_in", LoggedIn)
	mux.HandleFunc("/api/connect", Connect)
	mux.HandleFunc("/api/disconnect", Disconnect)
	mux.HandleFunc("/api/upload", Upload)
	mux.HandleFunc("/api/getuploads", GetUploads)
	mux.HandleFunc("/v/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(options.HtdocsDir, "index.html"))
	})
	mux.Handle("/", http.FileServer(http.Dir(options.HtdocsDir)))

	httpsrv := &http.Server{Handler: Logger(mux), Addr: options.Addr}
	if err := httpsrv.ListenAndServe(); err != nil {
		xlog.Fatalf("ListenAndServe: %v", err)
	}
}
