package main

import (
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"labix.org/v2/mgo/bson"
	"net/http"
)

func GetUploads(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)
	gplusID := session.Values["gplusID"]

	var result []interface{}

	coll := mongoDB.C("uploads")

	if err := coll.Find(bson.M{"owner": gplusID}).Sort("-uploaded").All(&result); err != nil {
		xlog.Errorf("Couldn't query uploads: %v", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
