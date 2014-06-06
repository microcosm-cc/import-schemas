package accounting

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

// IsImported checks if the old_id has already been imported for the given item type and
// returns the new item ID if so.
func ImportedItemID(db *sql.DB, originID int64, itemTypeID int64, oldID int64) (int64, error) {

	var itemID int64
	err := db.QueryRow(
		`SELECT item_id FROM imported_items WHERE origin_id = $1 AND item_type_id = $2 AND old_id = $3;`,
		originID,
		itemTypeID,
		oldID,
	).Scan(&itemID)

	return itemID, err
}

// This will load all item IDs from the imported_items table for a given site ID and item type ID.
// Potentially very expensive, use with care.
func LoadPriorImports(db *sql.DB, originID int64, itemTypeID int64) (IDmap map[int64]int64, err error) {

	rows, err := db.Query(
		`SELECT old_id, item_id FROM imported_items WHERE origin_id = $1 AND item_type = $2;`,
		originID,
		itemTypeID,
	)
	if err != nil {
		// No rows is not a problem.
		if err == sql.ErrNoRows {
			return IDmap, nil
		} else {
			return IDmap, err
		}
	}

	for rows.Next() {
		var oldID, newID int64
		if err := rows.Scan(&oldID, &newID); err != nil {
			return IDmap, err
		}
		IDmap[oldID] = newID
	}
	err = rows.Err()

	return IDmap, err
}
