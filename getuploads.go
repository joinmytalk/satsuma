package main

import (
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"github.com/russross/meddler"
	"net/http"
)

func GetUploads(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)
	gplusID := session.Values["gplusID"]

	var result []Upload
	if err := meddler.QueryAll(sqlDB, "uploads", "select * from uploads where owner = ?", gplusID); err != nil {
		xlog.Errorf("Couldn't query uploads: %v", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
