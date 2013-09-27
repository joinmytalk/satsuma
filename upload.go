package main

import (
	"encoding/json"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"github.com/surma-dump/gouuid"
	"net/http"
	"time"
)

type Upload struct {
	ID       int       `meddler:"id,pk" json:"-"`
	Title    string    `meddler:"title" json:"title"`
	PublicID string    `meddler:"public_id" json:"_id"`
	UserID   int       `meddler:"user_id" json:"-"`
	Uploaded time.Time `meddler:"uploaded,utctimez"`
}

type UploadHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
	UploadStore  *FileUploadStore
	SecureCookie *securecookie.SecureCookie
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	/* // re-enable as soon as ng-upload can do that.
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}
	*/

	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		http.Error(w, "couldn't parse form", http.StatusInternalServerError)
		return
	}

	StatCount("upload presentation", 1)

	title := r.FormValue("title")
	file, fhdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "couldn't read form", http.StatusInternalServerError)
		return
	}

	id := generateID()

	if err := h.UploadStore.Store(id, file, fhdr.Filename); err != nil {
		xlog.Errorf("Storing file for upload %s failed: %v", id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.DBStore.InsertUpload(&Upload{
		PublicID: id,
		UserID:   session.Values["userID"].(int),
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

type DeleteUploadHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
	UploadStore  *FileUploadStore
	SecureCookie *securecookie.SecureCookie
}

func (h *DeleteUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	StatCount("delete upload", 1)

	requestData := struct {
		UploadID string `json:"upload_id"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rowsAffected, err := h.DBStore.DeleteUploadByPublicID(requestData.UploadID, session.Values["userID"].(int))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected > 0 {
		h.UploadStore.Remove(requestData.UploadID)
	}

	w.WriteHeader(http.StatusNoContent)
}

type RenameUploadHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
	SecureCookie *securecookie.SecureCookie
}

func (h *RenameUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !VerifyXSRFToken(w, r, h.SessionStore, h.SecureCookie) {
		return
	}
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	if session.Values["userID"] == nil {
		http.Error(w, "authentication required", http.StatusForbidden)
		return
	}

	requestData := struct {
		UploadID string `json:"upload_id"`
		NewTitle string `json:"new_title"`
	}{}

	StatCount("rename upload", 1)

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.DBStore.SetTitleForPresentation(requestData.NewTitle, requestData.UploadID, session.Values["userID"].(int)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type GetUploadsHandler struct {
	SessionStore sessions.Store
	DBStore      *Store
}

func (h *GetUploadsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Debugf("Getting session failed: %v", err)
		StatCount("getting session failed", 1)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	userID := session.Values["userID"].(int)

	StatCount("get uploads", 1)

	result, err := h.DBStore.GetUploadsForUser(userID)
	if err != nil {
		xlog.Errorf("Couldn't query uploads: %v", err)
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func generateID() string {
	return gouuid.New().ShortString()
}
