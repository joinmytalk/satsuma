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
	Started  time.Time `meddler:"started" json:"started"`
	Ended    time.Time `meddler:"ended" json:"ended,omitempty"`
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

	if err := meddler.QueryRow(sqlDB, &uploadEntry, "select id from uploads where upload_id = ? and owner = ?", data.UploadID, session.Values["gplusID"]); err != nil {
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

	var result []struct {
		PublicID string    `meddler:"public_id" json:"_id"`
		Title    string    `meddler:"title" json:"title"`
		Started  time.Time `meddler:"started" json:"started"`
		Ended    time.Time `meddler:"ended" json:"ended,omitempty"`
	}

	if err := meddler.QueryAll(sqlDB, &result, "select sessions.public_id as public_id, sessions.started as started, sessions.ended as ended, uploads.title as title  from uploads, sessions where sessions.upload_id = uploads.id and uploads.owner = ? order by sessions.started desc", session.Values["gplusID"]); err != nil {
		xlog.Errorf("Querying sessions failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}