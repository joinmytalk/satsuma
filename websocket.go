package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/sessions"
	"github.com/joinmytalk/xlog"
	"time"
)

func WebsocketHandler(s *websocket.Conn, dbStore *Store, sessionStore sessions.Store, redisAddr string) {
	xlog.Infof("WebsocketHandler: opened connection")
	r := s.Request()
	session, _ := sessionStore.Get(r, SESSIONNAME)

	sessionData := struct {
		SessionID string `json:"session_id"`
	}{}

	if err := websocket.JSON.Receive(s, &sessionData); err != nil {
		xlog.Errorf("WebsocketHandler: JSON.Receive failed: %v", err)
		return
	}

	owner, sessionID, err := dbStore.GetOwnerForSession(sessionData.SessionID)
	if err != nil {
		xlog.Errorf("GetOwnerForSession failed: %v", err)
		return
	}

	if session.Values["userID"] == nil {
		xlog.Errorf("WebsocketHandler is not authenticated -> slave handler")
		slaveHandler(s, sessionID, dbStore, redisAddr)
	} else if owner == session.Values["userID"].(string) {
		xlog.Infof("WebSocketHandler owner matches -> master handler")
		masterHandler(s, sessionID, dbStore, redisAddr)
	} else {
		xlog.Infof("WebSocketHandler owner doesn't match -> slave handler")
		slaveHandler(s, sessionID, dbStore, redisAddr)
	}
}

type Command struct {
	ID           int       `meddler:"id,pk" json:"-"`
	Timestamp    time.Time `meddler:"timestamp,utctimez" json:"timestamp"`
	Cmd          string    `meddler:"cmd" json:"cmd"`
	Page         int       `meddler:"page" json:"page"`
	SessionID    int       `meddler:"session_id" json:"-"`
	Coordinates  []int     `meddler:"coordinates,json" json:"coords"`
	Color        string    `meddler:"color" json:"color"`
	Width        int       `meddler:"width" json:"width"`
	CanvasWidth  int       `meddler:"canvas_width" json:"canvasWidth"`
	CanvasHeight int       `meddler:"canvas_height" json:"canvasHeight"`
}

func slaveHandler(s *websocket.Conn, sessionID int, dbStore *Store, redisAddr string) {
	xlog.Debugf("entering SlaveHandler")
	c, err := redis.Dial("tcp", redisAddr)
	if err != nil {
		xlog.Errorf("redis.Dial failed: %v", err)
		return
	}
	defer c.Close()

	psc := redis.PubSubConn{Conn: c}
	topic := fmt.Sprintf("session.%d", sessionID)
	psc.Subscribe(topic)
	defer psc.Unsubscribe(topic)

	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			var cmd Command
			if err := json.Unmarshal(v.Data, &cmd); err != nil {
				break
			}
			if err := websocket.JSON.Send(s, cmd); err != nil {
				xlog.Errorf("slaveHandler: JSON.Send failed: %v", err)
				return
			}
			if cmd.Cmd == "close" {
				return
			}
		case redis.Subscription:
			xlog.Debugf("mkay... redis.Subscription received: %#v", v)
		}
	}
}

func masterHandler(s *websocket.Conn, sessionID int, dbStore *Store, redisAddr string) {
	xlog.Debugf("entering MasterHandler")
	c, err := redis.Dial("tcp", redisAddr)
	if err != nil {
		xlog.Errorf("redis.Dial failed: %v", err)
		return
	}
	defer c.Close()

	for {
		var cmd Command
		if err := websocket.JSON.Receive(s, &cmd); err != nil {
			xlog.Errorf("masterHandler: JSON.Receive failed: %v", err)
			break
		}

		xlog.Debugf("masterHandler: received command: %#v", cmd)

		cmd.SessionID = sessionID
		cmd.Timestamp = time.Now()

		if cmd.Cmd != "clearSlide" {
			if err := dbStore.InsertCommand(&cmd); err != nil {
				xlog.Errorf("Inserting command failed: %v", err)
				break
			}
		}

		executeCommand(cmd, dbStore)

		cmdJSON, _ := json.Marshal(cmd)

		c.Send("PUBLISH", fmt.Sprintf("session.%d", sessionID), string(cmdJSON))
		c.Flush()
	}
	xlog.Debugf("masterHandler: closing connection")
}

func executeCommand(cmd Command, dbStore *Store) {
	switch cmd.Cmd {
	case "clearSlide":
		if err := dbStore.ClearSlide(cmd.SessionID, cmd.Page); err != nil {
			xlog.Errorf("clearSlide for %d page %d failed: %v", cmd.SessionID, cmd.Page, err)
		}
	}
}
