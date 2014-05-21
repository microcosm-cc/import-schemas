package main

import (
	"database/sql"
)

func CreateImportOrigin(db *sql.DB, title string, siteId int64) (int64, error) {

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		return 0, err
	}

	var originId int64
	err = tx.QueryRow(
		`INSERT INTO import_origins (title, site_id) VALUES ($1, $2) RETURNING origin_id`,
		title,
		siteId,
	).Scan(&originId)
	if err != nil {
		return originId, err
	}
	return originId, tx.Commit()
}

func RecordImport(db *sql.DB, originId int64, itemTypeId int64, oldId int64, itemId int64) error {

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`INSERT into imported_items (origin_id, item_type_id, old_id, item_id) VALUES ($1, $2, $3, $4)`,
		originId,
		itemTypeId,
		oldId,
		itemId,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}
