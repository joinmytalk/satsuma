package main

import (
	"database/sql"
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

func (s *Store) GetUploadByPublicID(publicID string, userID string) (*Upload, error) {
	uploadEntry := &Upload{}

	err := meddler.QueryRow(s.sqlDB, uploadEntry, "select id from uploads where public_id = ? and owner = ?", publicID, userID)
	if err != nil {
		uploadEntry = nil
	}
	return uploadEntry, err
}

func (s *Store) InsertSession(sess *Session) error {
	return meddler.Insert(s.sqlDB, "sessions", sess)
}

func (s *Store) DeleteUploadByPublicID(publicID string, userID string) (int64, error) {
	result, err := s.sqlDB.Exec("DELETE FROM uploads WHERE public_id = ? AND owner = ?", publicID, userID)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

func (s *Store) GetUploadsForUser(userID string) ([]*Upload, error) {
	result := []*Upload{}
	err := meddler.QueryAll(s.sqlDB, &result, "SELECT * FROM uploads WHERE owner = ?", userID)
	if err != nil {
		result = nil
	}
	return result, err
}

func (s *Store) GetSessions(userID string) ([]*SessionData, error) {
	result := []*SessionData{}
	err := meddler.QueryAll(s.sqlDB, &result,
		`SELECT sessions.public_id AS public_id, 
			sessions.started AS started, 
			sessions.ended AS ended, 
			uploads.title AS title
		FROM uploads, sessions 
		WHERE sessions.upload_id = uploads.id AND 
			uploads.owner = ? 
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

func (s *Store) GetSessionInfoByPublicID(publicID, userID string) (*SessionInfo, error) {
	result := &SessionInfo{}
	err := meddler.QueryRow(s.sqlDB, result,
		`SELECT 
			uploads.title AS title, 
			uploads.public_id AS public_id, 
			uploads.owner AS owner
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
	result.IsOwner = (userID != "" && result.Owner == userID)

	return result, err
}

func (s *Store) GetOwnerForSession(publicID string) (owner string, sessionID int, err error) {
	ownerData := struct {
		Owner string `meddler:"owner"`
		ID    int    `meddler:"session_id"`
	}{}
	err = meddler.QueryRow(s.sqlDB, &ownerData, "SELECT uploads.owner AS owner, sessions.id AS session_id FROM uploads, sessions WHERE sessions.public_id = ? AND sessions.upload_id = uploads.id LIMIT 1", publicID)
	return ownerData.Owner, ownerData.ID, err
}

func (s *Store) StopSession(publicID string) {
	s.sqlDB.Exec("UPDATE sessions SET ended = NOW() WHERE public_id = ?", publicID)
}

func (s *Store) DeleteSession(publicID string) {
	s.sqlDB.Exec("DELETE FROM sessions WHERE public_id = ?", publicID)
}

func (s *Store) SetTitleForPresentation(title, publicID, userID string) error {
	_, err := s.sqlDB.Exec("UPDATE uploads SET title = ? WHERE public_id = ? AND owner = ?", title, publicID, userID)
	return err
}

func (s *Store) InsertCommand(cmd *Command) error {
	return meddler.Insert(s.sqlDB, "commands", cmd)
}

func (s *Store) ClearSlide(sessionID, page int) error {
	_, err := s.sqlDB.Exec("DELETE FROM commands WHERE session_id = ? AND page = ? AND cmd != 'gotoPage'", sessionID, page)
	return err
}
