package accounting

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/cheggaaa/pb"
	"github.com/golang/glog"

	h "github.com/microcosm-cc/microcosm/helpers"
)

// Tracks imported items
// The structure of these is all:
//   item[oldid] = newid
// Absence from this means that the item has not been imported
var (
	microcosms        map[int64]int64
	microcosmsLock    sync.Mutex
	conversations     map[int64]int64
	conversationsLock sync.Mutex
	comments          map[int64]int64
	commentsLock      sync.Mutex

	// Note that this actually looks up old userIDs and returns profileIDs
	profiles     map[int64]int64
	profilesLock sync.Mutex
)

func init() {
	microcosms = map[int64]int64{}
	conversations = map[int64]int64{}
	comments = map[int64]int64{}
	profiles = map[int64]int64{}
}

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

	updateStateMap(itemTypeID, oldID, itemID)

	return nil
}

func AddDeletedProfileID(profileID int64) {
	updateStateMap(h.ItemTypes[h.ItemTypeProfile], 0, profileID)
}

func updateStateMap(
	itemTypeID int64,
	oldID int64,
	newID int64,
) {

	switch itemTypeID {
	case h.ItemTypes[h.ItemTypeMicrocosm]:
		microcosmsLock.Lock()
		microcosms[oldID] = newID
		microcosmsLock.Unlock()

	case h.ItemTypes[h.ItemTypeProfile]:
		profilesLock.Lock()
		profiles[oldID] = newID
		profilesLock.Unlock()

	case h.ItemTypes[h.ItemTypeConversation]:
		conversationsLock.Lock()
		conversations[oldID] = newID
		conversationsLock.Unlock()

	case h.ItemTypes[h.ItemTypeComment]:
		commentsLock.Lock()
		comments[oldID] = newID
		commentsLock.Unlock()

	default:
		glog.Fatal(fmt.Errorf("Not yet implemented for %d", itemTypeID))
	}
}

// GetNewID checks if the old_id has already been imported for the given
// item type and returns the new item ID if so.
func GetNewID(
	originID int64,
	itemTypeID int64,
	oldID int64,
) int64 {

	var itemID int64

	switch itemTypeID {
	case h.ItemTypes[h.ItemTypeMicrocosm]:
		microcosmsLock.Lock()
		if newID, ok := microcosms[oldID]; ok {
			itemID = newID
		}
		microcosmsLock.Unlock()

	case h.ItemTypes[h.ItemTypeProfile]:
		profilesLock.Lock()
		if newID, ok := profiles[oldID]; ok {
			itemID = newID
		}
		profilesLock.Unlock()

	case h.ItemTypes[h.ItemTypeConversation]:
		conversationsLock.Lock()
		if newID, ok := conversations[oldID]; ok {
			itemID = newID
		}
		conversationsLock.Unlock()

	case h.ItemTypes[h.ItemTypeComment]:
		commentsLock.Lock()
		if newID, ok := comments[oldID]; ok {
			itemID = newID
		}
		commentsLock.Unlock()

	default:
		glog.Fatal(fmt.Errorf("Not yet implemented for %d", itemTypeID))
	}

	return itemID
}

// LoadPriorImports will load all item IDs from the imported_items table for a
// given site ID and item type ID.
// Potentially very expensive, use with care.
func LoadPriorImports(originID int64) {

	fmt.Println("Mapping existing records...")
	glog.Info("Mapping existing records...")

	db, err := h.GetConnection()
	if err != nil {
		glog.Errorf("Failed to get a DB connection: %+v", err)
		return
	}

	var records int
	err = db.QueryRow(`
SELECT COUNT(*)
 FROM imported_items
WHERE origin_id = $1`,
		originID,
	).Scan(
		&records,
	)
	if err != nil {
		glog.Fatal(err)
	}

	bar := pb.StartNew(records)

	rows, err := db.Query(`
SELECT item_type_id
      ,old_id::bigint
      ,item_id
 FROM imported_items
WHERE origin_id = $1`,
		originID,
	)
	if err != nil {
		glog.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		bar.Increment()

		var (
			itemTypeID int64
			oldID      int64
			newID      int64
		)
		err := rows.Scan(
			&itemTypeID,
			&oldID,
			&newID,
		)
		if err != nil {
			glog.Fatal(err)
		}

		updateStateMap(itemTypeID, oldID, newID)
	}
	err = rows.Err()
	if err != nil {
		glog.Fatal(err)
	}
	rows.Close()

	bar.Finish()

}
