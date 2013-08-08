package main

import (
	"bytes"
	"encoding/json"
	"github.com/joinmytalk/xlog"
	"io"
	"net/http"
)

func Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		http.Error(w, "couldn't parse form", http.StatusInternalServerError)
		return
	}

	title := r.FormValue("title")
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "couldn't read form", http.StatusInternalServerError)
		return
	}

	buf := &bytes.Buffer{}

	if _, err := io.Copy(buf, file); err != nil {
		xlog.Errorf("transferring file to buffer failed: %v", err)
	}

	xlog.Debugf("Upload: filename = %s title = %s (content is %d bytes long)", header.Filename, title, len(buf.Bytes()))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": "1234567890abcdef"})
}
