package main

import (
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"github.com/russross/meddler"
	"github.com/surma-dump/gouuid"
	"io"
	"net/http"
	"os"
	"path"
	"time"
)

type Upload struct {
	ID       int       `meddler:"id,pk" json:"-"`
	Title    string    `meddler:"title" json:"title"`
	PublicID string    `meddler:"pubid" json:"_id"`
	Owner    string    `meddler:"owner" `
	Uploaded time.Time `meddler:"uploaded"`
}

func DoUpload(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, SESSION_NAME)

	if session.Values["gplusID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

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

	id := generateID()

	filename := path.Join(options.UploadDir, id+".pdf")
	if f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		http.Error(w, "couldn't open file for writing", http.StatusInternalServerError)
		return
	} else {
		defer f.Close()

		if _, err := io.Copy(f, file); err != nil {
			xlog.Errorf("Writing file %s failed: %v", filename, err)
		}
	}

	if err := meddler.Insert(sqlDB, "uploads", &Upload{
		PublicID: id,
		Owner:    session.Values["gplusID"].(string),
		Title:    title,
		Uploaded: time.Now(),
	}); err != nil {
		xlog.Errorf("Insert failed: %v", err)
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func generateID() string {
	return gouuid.New().ShortString()
}
