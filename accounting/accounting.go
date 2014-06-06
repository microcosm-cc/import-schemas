package accounting

import (
	"database/sql"

	h "github.com/microcosm-cc/microcosm/helpers"
)

// CreateImportOrigin records in Postgres that we are about to start an import
// and the summary info of which site was the source of this
func CreateImportOrigin(tx *sql.Tx, title string, siteID int64) (int64, error) {

	var originID int64

	err := tx.QueryRow(`
INSERT INTO import_origins (
	title, site_id
) VALUES (
	$1, $2
) RETURNING origin_id`,
		title,
		siteID,
	).Scan(
		&originID,
	)
	if err != nil {
		return originID, err
	}

	return originID, nil
}

// RecordImport records a successful import of any item, this represents
// internally a map of identifiers from the source system to identifiers within
// Microcosm.
// i.e. we'd know that vBulletin Thread 321 == Microcosm Conversation 45984
func RecordImport(
	tx *sql.Tx,
	originID int64,
	itemTypeID int64,
	oldID int64,
	itemID int64,
) error {

	_, err := tx.Exec(`
INSERT into imported_items (
	origin_id, item_type_id, old_id, item_id
) VALUES (
	$1, $2, $3, $4
)`,
		originID,
		itemTypeID,
		oldID,
		itemID,
	)
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

// ImportedItemID checks if the old_id has already been imported for the given
// item type and returns the new item ID if so.
func ImportedItemID(
	originID int64,
	itemTypeID int64,
	oldID int64,
) (
	itemID int64,
	err error,
) {

	db, err := h.GetConnection()
	if err != nil {
		return
	}

	err = db.QueryRow(`
SELECT item_id
  FROM imported_items
 WHERE origin_id = $1
   AND item_type_id = $2
   AND old_id = $3`,
		originID,
		itemTypeID,
		oldID,
	).Scan(
		&itemID,
	)

	return
}

// LoadPriorImports will load all item IDs from the imported_items table for a
// given site ID and item type ID.
// Potentially very expensive, use with care.
func LoadPriorImports(
	originID int64,
	itemTypeID int64,
) (
	IDmap map[int64]int64,
	err error,
) {

	db, err := h.GetConnection()
	if err != nil {
		return
	}

	rows, err := db.Query(`
SELECT old_id
      ,item_id
 FROM imported_items
WHERE origin_id = $1
  AND item_type = $2`,
		originID,
		itemTypeID,
	)
	if err != nil {
		// No rows is not a problem.
		if err == sql.ErrNoRows {
			return IDmap, nil
		} else {
			return
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

	return
}
