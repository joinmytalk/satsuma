package main

import (
	"code.google.com/p/go.net/websocket"
	"github.com/joinmytalk/xlog"
	"github.com/russross/meddler"
	"time"
)

func WebsocketHandler(s *websocket.Conn) {
	xlog.Infof("WebsocketHandler: opened connection")
	r := s.Request()
	session, _ := store.Get(r, SESSION_NAME)

	sessionData := struct {
		SessionID string `json:"session_id"`
	}{}

	if err := websocket.JSON.Receive(s, &sessionData); err != nil {
		xlog.Errorf("WebsocketHandler: JSON.Receive failed: %v", err)
		return
	}

	ownerData := struct {
		Owner string `meddler:"owner"`
		ID    int    `meddler:"session_id"`
	}{}

	if err := meddler.QueryRow(sqlDB, &ownerData,
		"SELECT uploads.owner AS owner, sessions.id AS session_id FROM uploads, sessions WHERE uploads.id = sessions.upload_id AND sessions.public_id = ? LIMIT 1", sessionData.SessionID); err != nil {
		xlog.Errorf("meddler.QueryRow failed: %v", err)
		return
	}

	if session.Values["gplusID"] == nil {
		xlog.Errorf("WebsocketHandler is not authenticated -> slave handler")
		slaveHandler(s, ownerData.ID)
	} else if ownerData.Owner == session.Values["gplusID"].(string) {
		xlog.Infof("WebSocketHandler owner matches -> master handler")
		masterHandler(s, ownerData.ID)
	} else {
		xlog.Infof("WebSocketHandler owner doesn't match -> slave handler")
		slaveHandler(s, ownerData.ID)
	}
}

type Command struct {
	ID        int       `meddler:"id,pk" json:"-"`
	Timestamp time.Time `meddler:"timestamp,utctimez" json:"timestamp"`
	Cmd       string    `meddler:"cmd" json:"cmd"`
	Page      int       `meddler:"page" json:"page"`
	SessionID int       `meddler:"session_id" json:"-"`
}

func slaveHandler(s *websocket.Conn, sessionID int) {
	// TODO: implement
}

func masterHandler(s *websocket.Conn, sessionID int) {
	for {
		var cmd Command
		if err := websocket.JSON.Receive(s, &cmd); err != nil {
			xlog.Errorf("masterHandler: JSON.Receive failed: %v", err)
			break
		}

		xlog.Debugf("masterHandler: received command: %#v", cmd)

		cmd.SessionID = sessionID
		cmd.Timestamp = time.Now()

		if err := meddler.Insert(sqlDB, "commands", &cmd); err != nil {
			xlog.Errorf("Inserting command failed: %v", err)
			break
		}

		// TODO: broadcast to all slaves.
	}
	xlog.Infof("masterHandler: closing connection")
}
