package main

import (
	"bytes"
	"errors"
	"github.com/joinmytalk/xlog"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
)

// FileUploadStore abstracts the filesystem where files are uploaded to.
type FileUploadStore struct {
	UploadDir string
	TmpDir    string
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
		if err = ConvertFileToPDF(tmpFile, filename); err != nil {
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
	os.Remove(path.Join(store.UploadDir, uploadID+".pdf"))
}

// ConvertFileToPDF attempts to convert a file to PDF.
func ConvertFileToPDF(src, target string) error {
	StatCount("file conversion to PDF", 1)
	cmd := exec.Command("unoconv", "-f", "pdf", "--stdout", src)
	// cmd := exec.Command("cat", src)
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	defer f.Close()
	stdout, _ := cmd.StdoutPipe()
	err = cmd.Start()
	if err != nil {
		xlog.Errorf("running unoconv failed: %v", err)
		return err
	}
	io.Copy(f, stdout)
	err = cmd.Wait()
	if err != nil {
		xlog.Errorf("cmd.Wait returned error: %v", err)
		return err
	}
	fi, _ := f.Stat()
	if fi.Size() == 0 {
		os.Remove(target)
		xlog.Error("file resulting from conversion is empty")
		return errors.New("empty file")
	}

	return nil
}
