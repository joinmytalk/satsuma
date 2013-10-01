package main

import (
	"database/sql"
	"github.com/joinmytalk/xlog"
	"github.com/russross/meddler"
	"strings"
	"time"
)

// Store implements the higher-level operations on the data store.
type Store struct {
	sqlDB *sql.DB
}

// NewStore creates a new Store object from a database connection.
func NewStore(db *sql.DB) *Store {
	return &Store{sqlDB: db}
}

// InsertUpload inserts an Upload object into the uploads table.
func (s *Store) InsertUpload(u *Upload) error {
	return meddler.Insert(s.sqlDB, "uploads", u)
}

// GetUploadByPublicID returns an Upload object, identified by its
// publicID and userID.
func (s *Store) GetUploadByPublicID(publicID string, userID int) (*Upload, error) {
	uploadEntry := &Upload{}

	err := meddler.QueryRow(s.sqlDB, uploadEntry, "select id from uploads where public_id = ? and user_id = ?", publicID, userID)
	if err != nil {
		uploadEntry = nil
	}
	return uploadEntry, err
}

// InsertSession inserts a Session object into the sessions table.
func (s *Store) InsertSession(sess *Session) error {
	return meddler.Insert(s.sqlDB, "sessions", sess)
}

// DeleteUploadByPublicID deletes an upload, identified by its publicID and its userID.
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

// GetUploadsForUser returns a slice of Upload objects for the specified user.
func (s *Store) GetUploadsForUser(userID int) ([]*Upload, error) {
	result := []*Upload{}
	err := meddler.QueryAll(s.sqlDB, &result, "SELECT id, title, public_id, user_id, uploaded FROM uploads WHERE user_id = ?", userID)
	if err != nil {
		result = nil
	}
	return result, err
}

// GetSessions returns a slice of SessionData objects for the specified user.
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

// GetSessionInfoByPublicID returns a SessionInfo object for a session, identified
// by its publicID and userID.
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

// GetOwnerForSession returns the userID and numeric sessionID for a session, identified
// by its publicID.
func (s *Store) GetOwnerForSession(publicID string) (userID int, sessionID int, err error) {
	ownerData := struct {
		UserID int `meddler:"user_id"`
		ID     int `meddler:"session_id"`
	}{}
	err = meddler.QueryRow(s.sqlDB, &ownerData, "SELECT uploads.user_id AS user_id, sessions.id AS session_id FROM uploads, sessions WHERE sessions.public_id = ? AND sessions.upload_id = uploads.id LIMIT 1", publicID)
	return ownerData.UserID, ownerData.ID, err
}

// StopSession stops a session, identified by its publicID.
func (s *Store) StopSession(publicID string) {
	s.sqlDB.Exec("UPDATE sessions SET ended = NOW() WHERE public_id = ?", publicID)
}

// DeleteSession deletes a session, identified by its publicID.
func (s *Store) DeleteSession(publicID string) {
	s.sqlDB.Exec("DELETE FROM sessions WHERE public_id = ?", publicID)
}

// SetTitleForPresentation sets a new title for a presentation, identified
// by its publicID and userID.
func (s *Store) SetTitleForPresentation(title, publicID string, userID int) error {
	_, err := s.sqlDB.Exec("UPDATE uploads SET title = ? WHERE public_id = ? AND user_id = ?", title, publicID, userID)
	return err
}

// InsertCommand inserts a Command object into the commands table.
func (s *Store) InsertCommand(cmd *Command) error {
	return meddler.Insert(s.sqlDB, "commands", cmd)
}

// ClearSlide deletes all drawing-related commands for certain page of a session,
// identified by its sessionID.
func (s *Store) ClearSlide(sessionID, page int) error {
	_, err := s.sqlDB.Exec("DELETE FROM commands WHERE session_id = ? AND page = ? AND cmd != 'gotoPage'", sessionID, page)
	return err
}

// AddUser adds a new account (identified by username) to a user, identified by its
// userID.
func (s *Store) AddUser(username string, userID int) error {
	userData := []*struct {
		UserID int `meddler:"user_id"`
	}{}
	err := meddler.QueryAll(s.sqlDB, &userData, "SELECT user_id FROM accounts WHERE username = ? LIMIT 1", username)
	if err != nil {
		return err
	}

	if len(userData) > 0 {
		// account already logged in previously, migrate data to this user.

		// first, set account entries to current user.
		_, err := s.sqlDB.Exec("UPDATE accounts SET user_id = ? WHERE id = ?", userID, userData[0].UserID)
		if err != nil {
			return err
		}

		// then migrate uploads to current user.
		_, err = s.sqlDB.Exec("UPDATE uploads SET user_id = ? WHERE user_id = ?", userID, userData[0].UserID)
		if err != nil {
			return err
		}

		// finally, delete old user. ON DELETE CASCADE should clean up any old cruft.
		_, err = s.sqlDB.Exec("DELETE FROM users WHERE id = ?", userData[0].UserID)
	} else {
		// account is unknown, simply add new entry to accounts table.
		_, err := s.sqlDB.Exec("INSERT INTO accounts (username, user_id) VALUES (?, ?)", username, userID)
		if err != nil {
			return err
		}
	}

	return nil
}

// CreateUser checks whether an account for the specified username exists. If it
// does, then it returns its userID, otherwise it creates a new user and a new
// account with the specified username and links the account to the user.
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

// GetConnectedSystemsForUser returns a slice of auth service identifiers
// for which accounts exist that are associated with the specified userID.
func (s *Store) GetConnectedSystemsForUser(userID int) []string {
	systemMappings := map[string]string{
		"google.com":  "gplus",
		"twitter.com": "twitter",
	}

	connectedAccounts := []*struct {
		Username string `meddler:"username"`
	}{}

	if err := meddler.QueryAll(s.sqlDB, &connectedAccounts, "SELECT username FROM accounts WHERE user_id = ?", userID); err != nil {
		xlog.Errorf("Querying usernames for userID %d failed: %v", userID, err)
		return []string{}
	}

	systems := make([]string, 0, len(connectedAccounts))

	for _, acc := range connectedAccounts {
		systems = append(systems, systemMappings[strings.Split(acc.Username, ":")[0]])
	}

	return systems
}
