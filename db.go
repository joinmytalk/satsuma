package main

import (
	"database/sql"
	"github.com/joinmytalk/xlog"
	"github.com/russross/meddler"
	"time"
)

type Store struct {
	sqlDB *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{sqlDB: db}
}

func (s *Store) InsertUpload(u *Upload) error {
	return meddler.Insert(s.sqlDB, "uploads", u)
}

func (s *Store) GetUploadByPublicID(publicID string, userID int) (*Upload, error) {
	uploadEntry := &Upload{}

	err := meddler.QueryRow(s.sqlDB, uploadEntry, "select id from uploads where public_id = ? and user_id = ?", publicID, userID)
	if err != nil {
		uploadEntry = nil
	}
	return uploadEntry, err
}

func (s *Store) InsertSession(sess *Session) error {
	return meddler.Insert(s.sqlDB, "sessions", sess)
}

func (s *Store) DeleteUploadByPublicID(publicID string, userID int) (int64, error) {
	result, err := s.sqlDB.Exec("DELETE FROM uploads WHERE public_id = ? AND user_id = ?", publicID, userID)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

func (s *Store) GetUploadsForUser(userID int) ([]*Upload, error) {
	result := []*Upload{}
	err := meddler.QueryAll(s.sqlDB, &result, "SELECT id, title, public_id, user_id, uploaded FROM uploads WHERE user_id = ?", userID)
	if err != nil {
		result = nil
	}
	return result, err
}

func (s *Store) GetSessions(userID int) ([]*SessionData, error) {
	xlog.Debugf("GetSessions: userID = %d", userID)
	result := []*SessionData{}
	err := meddler.QueryAll(s.sqlDB, &result,
		`SELECT sessions.public_id AS public_id, 
			sessions.started AS started, 
			sessions.ended AS ended, 
			uploads.title AS title
		FROM uploads, sessions 
		WHERE sessions.upload_id = uploads.id AND 
			uploads.user_id = ? 
		ORDER BY sessions.started DESC`, userID)
	if err != nil {
		result = nil
	} else {
		// XXX: ugly hack.
		for _, entry := range result {
			formatted := entry.Ended.Format(time.RFC3339)
			if formatted != "0001-01-01T00:00:00Z" {
				entry.EndedJSON = formatted
			}
		}
	}
	return result, err
}

func (s *Store) GetSessionInfoByPublicID(publicID string, userID int) (*SessionInfo, error) {
	result := &SessionInfo{}
	err := meddler.QueryRow(s.sqlDB, result,
		`SELECT 
			uploads.title AS title, 
			uploads.public_id AS public_id, 
			uploads.user_id AS user_id
			FROM uploads, sessions
			WHERE sessions.upload_id = uploads.id AND
				sessions.public_id = ?`, publicID)
	if err != nil {
		return nil, err
	}
	err = meddler.QueryRow(s.sqlDB, &result,
		`SELECT 
			commands.page AS page
			FROM commands, sessions
			WHERE sessions.id = commands.session_id AND
				sessions.public_id = ?
			ORDER BY commands.timestamp DESC LIMIT 1`, publicID)
	if err != nil {
		result.Page = 1
	}

	var cmds []*Command

	err = meddler.QueryAll(s.sqlDB, &cmds,
		`SELECT
			*
			FROM commands
			WHERE commands.session_id = (SELECT id FROM sessions WHERE public_id = ?)
			ORDER BY commands.timestamp`, publicID)
	if err != nil {
		return nil, err
	}
	result.Cmds = cmds
	result.IsOwner = (userID != 0 && result.UserID == userID)

	return result, err
}

func (s *Store) GetOwnerForSession(publicID string) (userID int, sessionID int, err error) {
	ownerData := struct {
		UserID int `meddler:"user_id"`
		ID     int `meddler:"session_id"`
	}{}
	err = meddler.QueryRow(s.sqlDB, &ownerData, "SELECT uploads.user_id AS user_id, sessions.id AS session_id FROM uploads, sessions WHERE sessions.public_id = ? AND sessions.upload_id = uploads.id LIMIT 1", publicID)
	return ownerData.UserID, ownerData.ID, err
}

func (s *Store) StopSession(publicID string) {
	s.sqlDB.Exec("UPDATE sessions SET ended = NOW() WHERE public_id = ?", publicID)
}

func (s *Store) DeleteSession(publicID string) {
	s.sqlDB.Exec("DELETE FROM sessions WHERE public_id = ?", publicID)
}

func (s *Store) SetTitleForPresentation(title, publicID string, userID int) error {
	_, err := s.sqlDB.Exec("UPDATE uploads SET title = ? WHERE public_id = ? AND user_id = ?", title, publicID, userID)
	return err
}

func (s *Store) InsertCommand(cmd *Command) error {
	return meddler.Insert(s.sqlDB, "commands", cmd)
}

func (s *Store) ClearSlide(sessionID, page int) error {
	_, err := s.sqlDB.Exec("DELETE FROM commands WHERE session_id = ? AND page = ? AND cmd != 'gotoPage'", sessionID, page)
	return err
}

func (s *Store) CreateUser(username string) (int, error) {
	userData := []*struct {
		UserID int `meddler:"user_id"`
	}{}
	err := meddler.QueryAll(s.sqlDB, &userData, "SELECT user_id FROM accounts WHERE username = ? LIMIT 1", username)
	if err != nil {
		return 0, err
	}

	if len(userData) > 0 {
		return userData[0].UserID, nil
	}

	result, err := s.sqlDB.Exec("INSERT INTO users (id) VALUES(NULL)")
	if err != nil {
		return 0, err
	}

	lastInsertID, _ := result.LastInsertId()

	_, err = s.sqlDB.Exec("INSERT INTO accounts (username, user_id) VALUES (?, ?)", username, lastInsertID)
	if err != nil {
		return 0, err
	}

	return int(lastInsertID), nil
}
