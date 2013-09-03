package main

import (
	"io"
	"net/http"
	"os"
	"path"
)

type FileUploadStore struct {
	UploadDir string
}

func (store *FileUploadStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, path.Join(store.UploadDir, r.URL.Path))
}

func (store *FileUploadStore) Store(id string, uploadedFile io.Reader) error {
	filename := path.Join(store.UploadDir, id+".pdf")
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	defer f.Close()

	if _, err := io.Copy(f, uploadedFile); err != nil {
		return err
	}

	return nil
}

func (store *FileUploadStore) Remove(uploadID string) {
	os.Remove(path.Join(store.UploadDir, uploadID+".pdf"))
}
