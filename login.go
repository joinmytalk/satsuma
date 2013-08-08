package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/joinmytalk/xlog"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func Connect(w http.ResponseWriter, r *http.Request) {
	// Ensure that the request is not a forgery and that the user sending this
	// connect request is the expected user
	session, err := store.Get(r, SESSION_NAME)
	if err != nil {
		xlog.Errorf("Error fetching session: %v", err)
		http.Error(w, "Error fetching session", 500)
		return
	}
	/*
		if r.FormValue("state") != session.Values["state"].(string) {
			http.Error(w, "Invalid state parameter", 401)
			return
		}
	*/

	// Normally, the state is a one-time token; however, in this example, we want
	// the user to be able to connect and disconnect without reloading the page.
	// Thus, for demonstration, we don't implement this best practice.
	// session.Values["state"] = nil

	// Setup for fetching the code from the request payload
	x, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading code in request body", 500)
		return
	}
	code := string(x)

	accessToken, idToken, err := exchange(code)
	if err != nil {
		http.Error(w, "Error exchanging code for access token", 500)
		return
	}
	gplusID, err := decodeIdToken(idToken)

	if err != nil {
		http.Error(w, "Error decoding ID token", 500)
		return
	}

	/*
		// Check if the user is already connected
		storedToken := session.Values["accessToken"]
		storedGPlusID := session.Values["gplusID"]
		if storedToken != nil && storedGPlusID == gplusID {
			http.Error(w, "Current user already connected", 200)
			return
		}
	*/

	xlog.Debugf("Connect: gplusID = %s", gplusID)
	xlog.Debugf("Connect: accessToken = %s", accessToken)

	// Store the access token in the session for later use
	session.Values["accessToken"] = accessToken
	session.Values["gplusID"] = gplusID
	session.Save(r, w)
}

// randomString returns a random string with the specified length
func randomString(length int) (str string) {
	b := make([]byte, length)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func exchange(code string) (accessToken string, idToken string, err error) {
	// Exchange the authorization code for a credentials object via a POST request
	addr := "https://accounts.google.com/o/oauth2/token"
	values := url.Values{
		"Content-Type":  {"application/x-www-form-urlencoded"},
		"code":          {code},
		"client_id":     {options.ClientID},
		"client_secret": {options.ClientSecret},
		"redirect_uri":  {"postmessage"},
		"grant_type":    {"authorization_code"},
	}
	resp, err := http.PostForm(addr, values)
	if err != nil {
		return "", "", fmt.Errorf("Exchanging code: %v", err)
	}
	defer resp.Body.Close()

	// Decode the response body into a token object
	var token Token
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return "", "", fmt.Errorf("Decoding access token: %v", err)
	}

	return token.AccessToken, token.IdToken, nil
}

// decodeIdToken takes an ID Token and decodes it to fetch the Google+ ID within
func decodeIdToken(idToken string) (gplusID string, err error) {
	// An ID token is a cryptographically-signed JSON object encoded in base 64.
	// Normally, it is critical that you validate an ID token before you use it,
	// but since you are communicating directly with Google over an
	// intermediary-free HTTPS channel and using your Client Secret to
	// authenticate yourself to Google, you can be confident that the token you
	// receive really comes from Google and is valid. If your server passes the ID
	// token to other components of your app, it is extremely important that the
	// other components validate the token before using it.
	var set ClaimSet
	if idToken != "" {
		// Check that the padding is correct for a base64decode
		parts := strings.Split(idToken, ".")
		if len(parts) < 2 {
			return "", fmt.Errorf("Malformed ID token")
		}
		// Decode the ID token
		b, err := base64Decode(parts[1])
		if err != nil {
			return "", fmt.Errorf("Malformed ID token: %v", err)
		}
		err = json.Unmarshal(b, &set)
		if err != nil {
			return "", fmt.Errorf("Malformed ID token: %v", err)
		}
	}
	return set.Sub, nil
}

func base64Decode(s string) ([]byte, error) {
	// add back missing padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

func Disconnect(w http.ResponseWriter, r *http.Request) {
	// Only disconnect a connected user
	session, err := store.Get(r, SESSION_NAME)
	if err != nil {
		xlog.Error("Error fetching session: %v", err)
		http.Error(w, "Error fetching session", 500)
		return
	}
	token := session.Values["accessToken"]
	if token == nil {
		http.Error(w, "Current user not connected", 401)
		return
	}

	// Execute HTTP GET request to revoke current token
	url := "https://accounts.google.com/o/oauth2/revoke?token=" + token.(string)
	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Failed to revoke token for a given user", 400)
		return
	}
	defer resp.Body.Close()

	// Reset the user's session
	session.Values["accessToken"] = nil
	session.Save(r, w)
}
