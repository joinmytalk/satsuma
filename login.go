package main

import (
	"encoding/json"
	"github.com/bradrydzewski/go.auth"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"net/http"
)

func Connect(w http.ResponseWriter, r *http.Request, u auth.User, sessionStore sessions.Store, secureCookie *securecookie.SecureCookie, dbStore *Store) {
	StatCount("connect call", 1)
	session, err := sessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		session, _ = sessionStore.New(r, SESSIONNAME)
	}

	if userID, ok := session.Values["userID"].(int); ok {
		xlog.Debugf("Connect: already logged in (userID = %d), connecting account", userID)
		// we have a valid session -> connect account to user
		username := u.Provider() + ":" + u.Id()

		err := dbStore.AddUser(username, userID)
		if err != nil {
			xlog.Errorf("Error adding user: %v", err)
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		w.Header().Set("Location", "/settings")
	} else {
		xlog.Debugf("Connect: not logged in, actually log in user.")
		// no valid session -> actually login user
		username := u.Provider() + ":" + u.Id()
		xlog.Debugf("Connect: username = %s", username)
		userID, err := dbStore.CreateUser(username)
		if err != nil {
			xlog.Errorf("Error creating user: %v", err)
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		xlog.Debugf("Connect: userID = %d", userID)

		// set session values
		session.Values["userID"] = userID
		session.Values["username"] = username
		session.Values["email"] = u.Email()
		session.Values["name"] = u.Name()
		session.Save(r, w)

		// set XSRF-TOKEN for AngularJS
		xsrftoken, _ := secureCookie.Encode(XSRFTOKEN, username)
		http.SetCookie(w, &http.Cookie{Name: XSRFTOKEN, Value: xsrftoken, Path: "/"})

		w.Header().Set("Location", "/")
	}
	w.WriteHeader(http.StatusFound)
}

func VerifyXSRFToken(w http.ResponseWriter, r *http.Request, sessionStore sessions.Store, secureCookie *securecookie.SecureCookie) bool {
	xsrftoken := r.Header.Get(XSRFTOKENHEADER)
	userID := ""

	err := secureCookie.Decode(XSRFTOKEN, xsrftoken, &userID)
	if err == nil {
		session, _ := sessionStore.Get(r, SESSIONNAME)

		if userID != "" && userID == session.Values["username"].(string) {
			xlog.Infof("XSRF verification success for user %s", session.Values["username"].(string))
			return true
		}
		xlog.Errorf("XSRF issue: userID = %s session = %s", userID, session.Values["username"].(string))
	}

	xlog.Errorf("XSRF verification failed: %v (Request: %#v", err, *r)
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	StatCount("XSRF verification failed", 1)
	return false
}

type DisconnectHandler struct {
	SessionStore sessions.Store
	SecureCookie *securecookie.SecureCookie
	DBStore      *Store
}

func (h *DisconnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}

	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		http.Error(w, "Error fetching session", http.StatusInternalServerError)
		return
	}

	userID, ok := session.Values["userID"].(int)
	if !ok {
		http.Error(w, "Current user not logged in", http.StatusUnauthorized)
		return
	}

	postData := struct {
		Account string `json:"account"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&postData); err != nil {
		xlog.Errorf("DisconnectHandler: decoding POST body failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ok := h.DBStore.DisconnectAccountForUserID(userID, postData.Account); !ok {
		xlog.Errorf("DisconnectAccountForUserID failed.")
		http.Error(w, "disconnect failed", http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type LogoutHandler struct {
	SessionStore sessions.Store
	SecureCookie *securecookie.SecureCookie
}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	StatCount("logout call", 1)
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}

	// Only log out a connected user
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		http.Error(w, "Error fetching session", http.StatusInternalServerError)
		return
	}
	token := session.Values["username"]
	if token == nil {
		http.Error(w, "Current user not logged in", http.StatusUnauthorized)
		return
	}

	// Reset the user's session
	session.Values["userID"] = nil
	session.Values["username"] = nil
	session.Values["name"] = nil
	session.Values["email"] = nil
	session.Save(r, w)
	w.WriteHeader(http.StatusNoContent)
}

type LoggedInHandler struct {
	SessionStore sessions.Store
}

func (h *LoggedInHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	StatCount("loggedin call", 1)
	jsonEncoder := json.NewEncoder(w)
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		jsonEncoder.Encode(map[string]bool{"logged_in": false})
		xlog.Errorf("Error fetching session: %v", err)
		return
	}

	loggedIn := false
	username, ok := session.Values["username"].(string)
	if ok {
		loggedIn = true
	}

	w.Header().Set("Content-Type", "application/json")
	jsonEncoder.Encode(map[string]interface{}{"logged_in": loggedIn, "username": username})
}

type ConnectedHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
}

func (h *ConnectedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	userID := session.Values["userID"].(int)

	systems := h.DBStore.GetConnectedSystemsForUser(userID)

	jsonData := make(map[string]bool)

	for _, s := range systems {
		jsonData[s] = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonData)
}
