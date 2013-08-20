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
	xlog.Debugf("entering SlaveHandler")
	cmdChan := make(chan Command)
	registerCommandChannel(sessionID, cmdChan)
	defer unregisterCommandChannel(sessionID, cmdChan)
	for cmd := range cmdChan {
		if cmd.Cmd == "close" {
			return
		}
		if err := websocket.JSON.Send(s, cmd); err != nil {
			xlog.Errorf("slaveHandler: JSON.Send failed: %v", err)
			return
		}
	}
}

func masterHandler(s *websocket.Conn, sessionID int) {
	xlog.Debugf("entering MasterHandler")
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

		broadcastCommand(cmd)
	}
	xlog.Infof("masterHandler: closing connection")
}

type CommandChannels map[chan Command]struct{}
type Sessions map[int]CommandChannels

var sessionSlaves = make(Sessions)
var sessionChans = make(map[int]chan Command)

func broadcastCommand(cmd Command) {
	cmdChan, ok := sessionChans[cmd.SessionID]
	if !ok {
		cmdChan = make(chan Command)
		sessionChans[cmd.SessionID] = cmdChan
		go dispatch(cmd.SessionID)
	}
	cmdChan <- cmd
}

func dispatch(sessionID int) {
	ch := sessionChans[sessionID]
	for cmd := range ch {
		chans := sessionSlaves[sessionID]
		for slaveChan, _ := range chans {
			slaveChan <- cmd
		}
		if cmd.Cmd == "close" {
			close(ch)
			return
		}
	}
}

func registerCommandChannel(sessionID int, cmdChan chan Command) {
	channels, ok := sessionSlaves[sessionID]
	if !ok {
		channels = make(CommandChannels)
		sessionSlaves[sessionID] = channels
	}

	channels[cmdChan] = struct{}{}
}

func unregisterCommandChannel(sessionID int, cmdChan chan Command) {
	if channels, ok := sessionSlaves[sessionID]; ok {
		delete(channels, cmdChan)
	}
}
