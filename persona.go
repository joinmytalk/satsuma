package main

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
)

type PersonaAuthHandler struct {
	Audience     string
	SessionStore sessions.Store
	DBStore      *Store
	SecureCookie *securecookie.SecureCookie
}

func (h *PersonaAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := h.SessionStore.Get(r, SESSIONNAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		session, _ = h.SessionStore.New(r, SESSIONNAME)
	}

	assertionData := struct {
		Assertion string `json:"assertion"`
	}{}

	if err := json.NewDecoder(r.Body).Decode(&assertionData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	form := url.Values{"assertion": []string{assertionData.Assertion}, "audience": []string{h.Audience}}

	xlog.Debugf("Verifying Persona assertion...")
	resp, err := http.PostForm("https://verifier.login.persona.org/verify", form)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	verifierResponse := struct {
		Status   string `json:"status"`
		Email    string `json:"email"`
		Audience string `json:"audience"`
		Expires  uint64 `json:"expires"`
		Issuer   string `json:"issuer"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&verifierResponse); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	xlog.Debugf("Verifier response: %#v", verifierResponse)

	if verifierResponse.Status != "okay" {
		http.Error(w, "Not authenticated", http.StatusForbidden)
		return
	}

	if userID, ok := session.Values["userID"].(int); ok {
		xlog.Debugf("Persona: already logged in (userID = %d), connecting account", userID)
		// we have a valid session -> connect account to user
		username := "persona:" + verifierResponse.Email

		err := h.DBStore.AddUser(username, userID)
		if err != nil {
			xlog.Errorf("Persona: error adding user: %v", err)
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		// TODO: maybe deliver some additional information?
	} else {
		username := "persona:" + verifierResponse.Email
		xlog.Debugf("Persona: username = %s", username)
		userID, err := h.DBStore.CreateUser(username)
		if err != nil {
			xlog.Errorf("Error creating user: %v", err)
			http.Error(w, err.Error(), http.StatusForbidden)
		}

		xlog.Debugf("Persona: userID = %d", userID)

		session.Values["userID"] = userID
		session.Values["username"] = username
		session.Values["email"] = verifierResponse.Email
		session.Save(r, w)

		xsrftoken, _ := h.SecureCookie.Encode(XSRFTOKEN, username)
		http.SetCookie(w, &http.Cookie{Name: XSRFTOKEN, Value: xsrftoken, Path: "/"})

		w.WriteHeader(http.StatusOK)
	}
}
