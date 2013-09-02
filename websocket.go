package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/joinmytalk/xlog"
	"github.com/russross/meddler"
	"time"
)

func WebsocketHandler(s *websocket.Conn) {
	xlog.Infof("WebsocketHandler: opened connection")
	r := s.Request()
	session, _ := store.Get(r, SESSIONNAME)

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

	if session.Values["userID"] == nil {
		xlog.Errorf("WebsocketHandler is not authenticated -> slave handler")
		slaveHandler(s, ownerData.ID)
	} else if ownerData.Owner == session.Values["userID"].(string) {
		xlog.Infof("WebSocketHandler owner matches -> master handler")
		masterHandler(s, ownerData.ID)
	} else {
		xlog.Infof("WebSocketHandler owner doesn't match -> slave handler")
		slaveHandler(s, ownerData.ID)
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

func slaveHandler(s *websocket.Conn, sessionID int) {
	xlog.Debugf("entering SlaveHandler")
	c, err := redis.Dial("tcp", options.RedisAddr)
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

func masterHandler(s *websocket.Conn, sessionID int) {
	xlog.Debugf("entering MasterHandler")
	c, err := redis.Dial("tcp", options.RedisAddr)
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
			if err := meddler.Insert(sqlDB, "commands", &cmd); err != nil {
				xlog.Errorf("Inserting command failed: %v", err)
				break
			}
		}

		executeCommand(cmd)

		cmdJSON, _ := json.Marshal(cmd)

		c.Send("PUBLISH", fmt.Sprintf("session.%d", sessionID), string(cmdJSON))
		c.Flush()
	}
	xlog.Debugf("masterHandler: closing connection")
}

func executeCommand(cmd Command) {
	switch cmd.Cmd {
	case "clearSlide":
		if _, err := sqlDB.Exec("DELETE FROM commands WHERE session_id = ? AND page = ? AND cmd != 'gotoPage'", cmd.SessionID, cmd.Page); err != nil {
			xlog.Errorf("clearSlide for %d page %d failed: %v", cmd.SessionID, cmd.Page, err)
		}
	}
}
