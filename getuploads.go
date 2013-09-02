package main

import (
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"net/http"
)

func GetUploads(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSIONNAME)
	userID := session.Values["userID"].(string)

	result, err := dbStore.GetUploadsForUser(userID)
	if err != nil {
		xlog.Errorf("Couldn't query uploads: %v", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
