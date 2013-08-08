package main

import (
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"io"
	"labix.org/v2/mgo/bson"
	"net/http"
	"os"
	"path"
	"time"
)

func Upload(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)
	xlog.Debugf("Session: %#v", session.Values)

	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		http.Error(w, "couldn't parse form", http.StatusInternalServerError)
		return
	}

	title := r.FormValue("title")
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "couldn't read form", http.StatusInternalServerError)
		return
	}

	id := bson.NewObjectId()

	filename := path.Join(options.UploadDir, id.Hex()+".pdf")
	if f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		http.Error(w, "couldn't open file for writing", http.StatusInternalServerError)
		return
	} else {
		defer f.Close()

		if _, err := io.Copy(f, file); err != nil {
			xlog.Errorf("Writing file %s failed: %v", filename, err)
		}
	}

	if err := mongoDB.C("uploads").Insert(bson.M{
		"_id":      id,
		"owner":    session.Values["gplusID"],
		"title":    title,
		"uploaded": time.Now(),
	}); err != nil {
		xlog.Errorf("Insert failed: %v", err)
		http.Error(w, "insert failed", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id.Hex()})
}
