package main

import (
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"net/http"
	"time"
)

type Session struct {
	ID       int       `meddler:"id,pk" json:"-"`
	UploadID int       `meddler:"upload_id" json:"-"`
	PublicID string    `meddler:"public_id" json:"_id"`
	Started  time.Time `meddler:"started,utctimez" json:"started"`
	Ended    time.Time `meddler:"ended,utctimez" json:"ended,omitempty"`
}

func StartSession(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSIONNAME)

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	data := struct {
		UploadID string `json:"upload_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		xlog.Errorf("Decoding POST body failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	uploadEntry, err := dbStore.GetUploadByPublicID(data.UploadID, session.Values["userID"].(string))
	if err != nil {
		xlog.Errorf("Querying upload %s failed: %v", data.UploadID, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	id := generateID()

	if err := dbStore.InsertSession(&Session{
		UploadID: uploadEntry.ID,
		PublicID: id,
		Started:  time.Now(),
	}); err != nil {
		xlog.Errorf("Insert failed: %v", err)
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// used by Store.GetSessions
type SessionData struct {
	PublicID  string    `meddler:"public_id" json:"_id"`
	Title     string    `meddler:"title" json:"title"`
	Started   time.Time `meddler:"started,utctimez" json:"started"`
	Ended     time.Time `meddler:"ended,utctimez" json:"-"`
	EndedJSON string    `meddler:"-" json:"ended,omitempty"`
}

func GetSessions(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSIONNAME)

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	result, err := dbStore.GetSessions(session.Values["userID"].(string))

	if err != nil {
		xlog.Errorf("Querying sessions failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type SessionInfo struct {
	Title    string     `meddler:"title" json:"title"`
	UploadID string     `meddler:"public_id" json:"upload_id"`
	IsOwner  bool       `json:"owner" meddler:"-"`
	Owner    string     `meddler:"owner" json:"-"`
	Page     int        `meddler:"page" json:"page"`
	Cmds     []*Command `meddler:"-" json:"cmds"`
}

func GetSessionInfo(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSIONNAME)

	userID := ""
	if session.Values["userID"] != nil {
		userID = session.Values["userID"].(string)
	}

	publicID := r.URL.Query().Get(":id")

	result, err := dbStore.GetSessionInfoByPublicID(publicID, userID)
	if err != nil {
		xlog.Errorf("Loading session information failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func StopSession(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSIONNAME)

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	requestData := struct {
		PublicID string `json:"session_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	owner, _, err := dbStore.GetOwnerForSession(requestData.PublicID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if owner != session.Values["userID"].(string) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	dbStore.StopSession(requestData.PublicID)

	w.WriteHeader(http.StatusNoContent)
}

func DeleteSession(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSIONNAME)

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	requestData := struct {
		PublicID string `json:"session_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	owner, _, err := dbStore.GetOwnerForSession(requestData.PublicID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if owner != session.Values["userID"].(string) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	dbStore.DeleteSession(requestData.PublicID)

	w.WriteHeader(http.StatusNoContent)
}
