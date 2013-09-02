package main

import (
	"encoding/json"
	"github.com/bradrydzewski/go.auth"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"net/http"
)

func Connect(w http.ResponseWriter, r *http.Request, u auth.User, sessionStore sessions.Store) {
	session, err := sessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		http.Error(w, "Error fetching session", 500)
		return
	}

	userID := u.Provider() + ":" + u.Id()
	xlog.Debugf("Connect: userID = %s", userID)

	// Store the access token in the session for later use
	session.Values["userID"] = userID
	session.Values["email"] = u.Email()
	session.Values["name"] = u.Name()
	session.Save(r, w)
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusFound)
}

type DisconnectHandler struct {
	SessionStore sessions.Store
}

func (h *DisconnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
