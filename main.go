package main

import (
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"github.com/voxelbrain/goptions"
	"net/http"
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

var store sessions.Store

var options = struct {
	Addr         string `goptions:"-L, --listen, description='Listen address'"`
	CookieKey    string `goptions:"-k, --key, description='Secret key for cookie store', obligatory"`
	ClientID     string `goptions:"--clientid, description='Client ID', obligatory"`
	ClientSecret string `goptions:"--clientsecret, description='Client Secret', obligatory"`
}{
	Addr: "[::]:8080",
}

func main() {
	goptions.ParseAndFail(&options)

	store = sessions.NewCookieStore([]byte(options.CookieKey))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/connect", Connect)
	mux.HandleFunc("/api/disconnect", Disconnect)
	mux.Handle("/", http.FileServer(http.Dir("htdocs")))

	httpsrv := &http.Server{Handler: Logger(mux), Addr: options.Addr}
	if err := httpsrv.ListenAndServe(); err != nil {
		xlog.Fatalf("ListenAndServe: %v", err)
	}
}
