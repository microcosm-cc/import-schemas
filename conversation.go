package main

import (
	"database/sql"
	"time"
)

type Conversation struct {
	ConversationId int64
	MicrocosmID    int64
	Title          string
	Created        time.Time
	CreatedBy      int64
	Edited         time.Time
	EditedBy       int64
	EditReason     string
	IsSticky       bool
	IsOpen         bool
	IsDeleted      bool
	IsModerated    bool
	IsVisible      bool
	CommentCount   int64
	ViewCount      int64
}

func StoreConversation(db *sql.DB, c Conversation) (int64, error) {

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		return 0, err
	}

	var cID int64
	err = tx.QueryRow(
		`INSERT INTO conversations (
            microcosm_id, title, created, created_by, is_sticky,
            is_open, is_deleted, is_moderated, is_visible
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9
        ) RETURNING conversation_id;`,
		c.MicrocosmID, c.Title, c.Created, c.CreatedBy, c.IsSticky, c.IsOpen,
		c.IsDeleted, c.IsModerated, c.IsVisible,
	).Scan(&cID)

	if err != nil {
		return cID, err
	}
	err = tx.Commit()
	return cID, err

}
