package main

import (
	"encoding/json"
	"github.com/bradrydzewski/go.auth"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"net/http"
)

func Connect(w http.ResponseWriter, r *http.Request, u auth.User, sessionStore sessions.Store, secureCookie *securecookie.SecureCookie) {
	session, err := sessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		session, _ = sessionStore.New(r, SESSIONNAME)
	}

	userID := u.Provider() + ":" + u.Id()
	xlog.Debugf("Connect: userID = %s", userID)

	// set session values
	session.Values["userID"] = userID
	session.Values["email"] = u.Email()
	session.Values["name"] = u.Name()
	session.Save(r, w)

	// set XSRF-TOKEN for AngularJS
	xsrftoken, _ := secureCookie.Encode(XSRFTOKEN, userID)
	http.SetCookie(w, &http.Cookie{Name: XSRFTOKEN, Value: xsrftoken, Path: "/"})

	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusFound)
}

func VerifyXSRFToken(w http.ResponseWriter, r *http.Request, sessionStore sessions.Store, secureCookie *securecookie.SecureCookie) bool {
	xsrftoken := r.Header.Get(XSRFTOKENHEADER)
	userID := ""

	err := secureCookie.Decode(XSRFTOKEN, xsrftoken, &userID)
	if err == nil {
		session, _ := sessionStore.Get(r, SESSIONNAME)

		if userID != "" && userID == session.Values["userID"].(string) {
			xlog.Infof("XSRF verification success for user %s", session.Values["userID"].(string))
			return true
		} else {
			xlog.Errorf("XSRF issue: userID = %s session = %s", userID, session.Values["userID"].(string))
		}
	}

	xlog.Errorf("XSRF verification failed: %v (Request: %#v", err, *r)
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	return false
}

type DisconnectHandler struct {
	SessionStore sessions.Store
	SecureCookie *securecookie.SecureCookie
}

func (h *DisconnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}

	// Only disconnect a connected user
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		http.Error(w, "Error fetching session", 500)
		return
	}
	token := session.Values["userID"]
	if token == nil {
		http.Error(w, "Current user not connected", 401)
		return
	}

	// Reset the user's session
	session.Values["userID"] = nil
	session.Values["name"] = nil
	session.Values["email"] = nil
	session.Save(r, w)
	w.WriteHeader(http.StatusNoContent)
}

type LoggedInHandler struct {
	SessionStore sessions.Store
}

func (h *LoggedInHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jsonEncoder := json.NewEncoder(w)
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		jsonEncoder.Encode(map[string]bool{"logged_in": false})
		xlog.Errorf("Error fetching session: %v", err)
		return
	}

	loggedIn := (session.Values["userID"] != nil)

	w.Header().Set("Content-Type", "application/json")
	jsonEncoder.Encode(map[string]bool{"logged_in": loggedIn})
}
