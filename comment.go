package main

import (
	"database/sql"
	"time"
)

type Comment struct {
	CommentId       int64
	ItemTypeId      int64
	ItemId          int64
	ProfileId       int64
	Created         time.Time
	InReplyTo       int64
	IsVisible       bool
	IsModerated     bool
	IsDeleted       bool
	AttachmentCount int64
}

type CommentRevision struct {
	RevisionId int64
	CommentId  int64
	ProfileId  int64
	Raw        string
	HTML       string
	Created    time.Time
	IsCurrent  bool
}

func StoreComment(db *sql.DB, c Comment) (int64, error) {

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		return 0, err
	}

	var cID int64
	err = tx.QueryRow(
		`INSERT INTO comments () VALUES () RETURNING conversation_id;`,
	).Scan(&cID)

	if err != nil {
		return cID, err
	}
	err = tx.Commit()
	return cID, err

}

func StoreRevision() {}
