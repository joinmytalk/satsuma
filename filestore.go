package main

import (
	"io"
	"os"
	"path"
)

type FileUploadStore struct {
	UploadDir string
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
