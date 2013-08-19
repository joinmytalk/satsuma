package main

import (
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"github.com/russross/meddler"
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
	session, _ := store.Get(r, SESSION_NAME)

	if session.Values["gplusID"] == nil {
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

	var uploadEntry Upload

	if err := meddler.QueryRow(sqlDB, &uploadEntry, "select id from uploads where public_id = ? and owner = ?", data.UploadID, session.Values["gplusID"]); err != nil {
		xlog.Errorf("Querying upload %s failed: %v", data.UploadID, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	id := generateID()

	if err := meddler.Insert(sqlDB, "sessions", &Session{
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

func GetSessions(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)

	if session.Values["gplusID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	result := []*struct {
		PublicID string    `meddler:"public_id" json:"_id"`
		Title    string    `meddler:"title" json:"title"`
		Started  time.Time `meddler:"started,utctimez" json:"started"`
		Ended    time.Time `meddler:"ended,utctimez" json:"ended,omitempty"`
	}{}

	if err := meddler.QueryAll(sqlDB, &result, "select sessions.public_id as public_id, sessions.started as started, sessions.ended as ended, uploads.title as title  from uploads, sessions where sessions.upload_id = uploads.id and uploads.owner = ? order by sessions.started desc", session.Values["gplusID"]); err != nil {
		xlog.Errorf("Querying sessions failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func SessionInfo(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)

	if session.Values["gplusID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	id := r.URL.Query().Get(":id")

	result := struct {
		Title    string `meddler:"title" json:"title"`
		UploadID string `meddler:"public_id" json:"upload_id"`
		IsOwner  bool   `json:"owner" meddler:"-"`
		Owner    string `meddler:"owner" json:"-"`
		//Page int `json:"page"`
	}{}

	if err := meddler.QueryRow(sqlDB, &result,
		`SELECT 
			uploads.title AS title, 
			uploads.public_id AS public_id, 
			uploads.owner AS owner
			FROM uploads, sessions
			WHERE sessions.upload_id = uploads.id AND
				sessions.public_id = ?`, id); err != nil {
		xlog.Errorf("Loading session information failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	result.IsOwner = (result.Owner == session.Values["gplusID"].(string))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func StopSession(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)

	if session.Values["gplusID"] == nil {
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

	ownerData := struct {
		Owner string `json:"owner"`
	}{}

	if err := meddler.QueryRow(sqlDB, &ownerData, "SELECT uploads.owner AS owner FROM uploads, sessions WHERE sessions.public_id = ? AND sessions.upload_id = uploads.id LIMIT 1", requestData.PublicID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ownerData.Owner != session.Values["gplusID"].(string) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sqlDB.Exec("UPDATE sessions SET ended = NOW() WHERE public_id = ?", requestData.PublicID)

	w.WriteHeader(http.StatusNoContent)
}

func DeleteSession(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)

	if session.Values["gplusID"] == nil {
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

	ownerData := struct {
		Owner string `meddler:"owner"`
	}{}

	if err := meddler.QueryRow(sqlDB, &ownerData, "SELECT uploads.owner AS owner FROM uploads, sessions WHERE sessions.public_id = ? AND sessions.upload_id = uploads.id LIMIT 1", requestData.PublicID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ownerData.Owner != session.Values["gplusID"].(string) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sqlDB.Exec("DELETE FROM sessions WHERE public_id = ?", requestData.PublicID)

	w.WriteHeader(http.StatusNoContent)
}
