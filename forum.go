package main

import (
	"database/sql"
	"time"
)

type Microcosm struct {
	Title       string
	Description string
	SiteId      int64
	Created     time.Time
	CreatedBy   int64
	OwnedBy     int64
	IsSticky    bool
	IsModerated bool
	IsOpen      bool
	IsDeleted   bool
	IsVisible   bool
}

func StoreMicrocosm(db *sql.DB, m Microcosm) (int64, error) {

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		return 0, err
	}

	var microcosmID int64
	err = tx.QueryRow(
		`INSERT INTO microcosms (
            title, description, site_id, created, created_by, owned_by,
            is_sticky, is_moderated, is_open, is_deleted, is_visible
        ) VALUES (
            $1, $2, $3, NOW(), $4, $5,
            $6, $7, $8, $9, $10
        ) RETURNING microcosm_id;`,
		m.Title, m.Description, m.SiteId, m.CreatedBy, m.OwnedBy,
		m.IsSticky, m.IsModerated, m.IsOpen, m.IsDeleted, m.IsVisible,
	).Scan(&microcosmID)

	if err != nil {
		return microcosmID, err
	}
	err = tx.Commit()
	return microcosmID, err
}
