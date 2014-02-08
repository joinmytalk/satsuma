package main

import (
	"bytes"
	"encoding/json"
	"github.com/bitly/go-nsq"
	"github.com/joinmytalk/xlog"
	"io"
	"net/http"
	"os"
	"path"
)

// FileUploadStore abstracts the filesystem where files are uploaded to.
type FileUploadStore struct {
	UploadDir string
	TmpDir    string
	NSQ       *nsq.Writer
	Topic     string
}

// ServeHTTP serves HTTP request from the FileUploadStore.
func (store *FileUploadStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, path.Join(store.UploadDir, r.URL.Path))
}

// Store stores a new file with a specified id in the filesystem. If the
// file isn't a PDF file, it also attempts a conversion to a PDF file.
func (store *FileUploadStore) Store(id string, uploadedFile io.Reader, origFileName string) error {
	filename := path.Join(store.UploadDir, id+".pdf")

	tmpFile := path.Join(store.TmpDir, id+"_"+origFileName)
	tmpf, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	io.Copy(tmpf, uploadedFile)
	tmpf.Close()

	f, err := os.Open(tmpFile)
	if err != nil {
		return err
	}

	defer f.Close()
	buf := make([]byte, 4)
	if _, err = f.Read(buf); err != nil {
		return err
	}

	if bytes.Equal(buf, []byte("%PDF")) {
		xlog.Debugf("%s is a PDF file, renaming to %s", tmpFile, filename)
		os.Rename(tmpFile, filename)
	} else {
		if err = store.ConvertFileToPDF(tmpFile, filename); err != nil {
			xlog.Errorf("conversion to PDF of %s failed: %v", tmpFile, err)
			os.Remove(tmpFile)
			os.Remove(filename)
			return err
		}
		os.Remove(tmpFile)
	}

	return nil
}

// Remove removes an uploaded file from the file store.
func (store *FileUploadStore) Remove(uploadID string) {
	filePath := path.Join(store.UploadDir, uploadID+".pdf")
	xlog.Debugf("FileUploadStore: remove %s", filePath)
	os.Remove(filePath)
}

// ConvertFileToPDF attempts to convert a file to PDF.
func (store *FileUploadStore) ConvertFileToPDF(src, target string) error {
	msg, _ := json.Marshal(map[string]string{"src_file": src, "target_file": target})
	if _, _, err := store.NSQ.Publish(store.Topic, msg); err != nil {
		xlog.Errorf("Queuing message to NSQ %s failed: %v", store.NSQ.Addr, err)
		return err
	}
	return nil
}
