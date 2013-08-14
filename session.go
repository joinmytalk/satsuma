package main

import (
	"encoding/json"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"net/http"
	"time"
)

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

	id := bson.NewObjectId()

	// TODO: add a check that UploadID is owned by user.
	if err := mongoDB.C("sessions").Insert(bson.M{
		"_id":     id,
		"upload":  bson.ObjectIdHex(data.UploadID),
		"started": time.Now(),
		"owner":   session.Values["gplusID"],
	}); err != nil {
		xlog.Errorf("Insert failed: %v", err)
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id.Hex()})
}

func GetSessions(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)

	if session.Values["gplusID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}
	// TODO: implement.
}
