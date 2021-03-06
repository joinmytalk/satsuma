package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
)

type Session struct {
	ID       int       `meddler:"id,pk" json:"-"`
	UploadID int       `meddler:"upload_id" json:"-"`
	PublicID string    `meddler:"public_id" json:"id"`
	Started  time.Time `meddler:"started,utctimez" json:"started"`
	Ended    time.Time `meddler:"ended,utctimez" json:"ended,omitempty"`
}

type StartSessionHandler struct {
	DBStore      *Store
	SessionStore sessions.Store
	SecureCookie *securecookie.SecureCookie
}

func (h *StartSessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}

	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	StatCount("start session", 1)

	data := struct {
		UploadID string `json:"upload_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		xlog.Errorf("Decoding POST body failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uploadEntry, err := h.DBStore.GetUploadByPublicID(data.UploadID, session.Values["userID"].(int))
	if err != nil {
		xlog.Errorf("Querying upload %s failed: %v", data.UploadID, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	id := generateID()

	if err := h.DBStore.InsertSession(&Session{
		UploadID: uploadEntry.ID,
		PublicID: id,
		Started:  time.Now().UTC(),
	}); err != nil {
		xlog.Errorf("Insert failed: %v", err)
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// SessionData is used by Store.GetSessions
type SessionData struct {
	PublicID  string    `meddler:"public_id" json:"id"`
	Title     string    `meddler:"title" json:"title"`
	Started   time.Time `meddler:"started,utctimez" json:"started"`
	Ended     time.Time `meddler:"ended,utctimez" json:"-"`
	EndedJSON string    `meddler:"-" json:"ended,omitempty"`
}

type GetSessionsHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
}

func (h *GetSessionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if session.Values["username"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	StatCount("get sessions", 1)

	result, err := h.DBStore.GetSessions(session.Values["userID"].(int))

	if err != nil {
		xlog.Errorf("Querying sessions failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type SessionInfo struct {
	Title     string     `meddler:"title" json:"title"`
	UploadID  string     `meddler:"public_id" json:"upload_id"`
	IsOwner   bool       `json:"owner" meddler:"-"`
	UserID    int        `meddler:"user_id" json:"-"`
	Page      int        `meddler:"page" json:"page"`
	Ended     time.Time  `meddler:"ended,utctimez" json:"-"`
	EndedJSON string     `meddler:"-" json:"ended,omitempty"`
	Cmds      []*Command `meddler:"-" json:"cmds"`
}

type GetSessionInfoHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
}

func (h *GetSessionInfoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	StatCount("session info", 1)
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	userID := 0
	if session.Values["userID"] != nil {
		userID = session.Values["userID"].(int)
	}

	publicID := r.URL.Query().Get(":id")

	result, err := h.DBStore.GetSessionInfoByPublicID(publicID, userID)
	if err != nil {
		xlog.Errorf("Loading session information failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type StopSessionHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
	SecureCookie *securecookie.SecureCookie
	RedisAddr    string
}

func (h *StopSessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	StatCount("stop session", 1)

	requestData := struct {
		PublicID string `json:"session_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ownerID, sessionID, err := h.DBStore.GetOwnerForSession(requestData.PublicID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ownerID != session.Values["userID"].(int) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.DBStore.StopSession(requestData.PublicID)

	w.WriteHeader(http.StatusNoContent)

	c, err := redis.Dial("tcp", h.RedisAddr)
	if err != nil {
		xlog.Errorf("redis.Dial failed: %v", err)
		return
	}
	defer c.Close()

	cmdJSON, _ := json.Marshal(&Command{
		Cmd:       "close",
		SessionID: sessionID,
		Timestamp: time.Now(),
	})

	c.Send("PUBLISH", fmt.Sprintf("session.%d", sessionID), string(cmdJSON))
	c.Flush()
}

type DeleteSessionHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
	SecureCookie *securecookie.SecureCookie
}

func (h *DeleteSessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	StatCount("delete session", 1)

	requestData := struct {
		PublicID string `json:"session_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ownerID, _, err := h.DBStore.GetOwnerForSession(requestData.PublicID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ownerID != session.Values["userID"].(int) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.DBStore.DeleteSession(requestData.PublicID)

	w.WriteHeader(http.StatusNoContent)
}
