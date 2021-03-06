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
	attachments       = make(map[int64]int64)
	attachmentsLock   sync.RWMutex
	conversations     = make(map[int64]int64)
	conversationsLock sync.RWMutex
	comments          = make(map[int64]int64)
	commentsLock      sync.RWMutex
	huddles           = make(map[int64]int64)
	huddlesLock       sync.RWMutex
	microcosms        = make(map[int64]int64)
	microcosmsLock    sync.RWMutex

	// Note that this actually looks up old userIDs and returns profileIDs
	profiles     = make(map[int64]int64)
	profilesLock sync.RWMutex

	roles        = make(map[int64]int64)
	rolesLock    sync.RWMutex
	watchers     = make(map[int64]int64)
	watchersLock sync.RWMutex
)

// CreateImportOrigin records in Postgres that we are about to start an import
// and the summary info of which site was the source of this
func CreateImportOrigin(tx *sql.Tx, title string, siteID int64) (int64, error) {

	var originID int64

	// TODO: Today this just presumes that we are a vBulletin 3.8 series forum
	// but in the future we need to make this accept whatever the exported data
	// tells us.
	err := tx.QueryRow(`
INSERT INTO import_origins (
	title, site_id, product, major_version, minor_version
) VALUES (
	$1, $2, 'vbulletin', 3, 8
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

func ImportFollows(originID int64) bool {
	db, err := h.GetConnection()
	if err != nil {
		return false
	}

	var importFollows bool

	db.QueryRow(`SELECT imported_follows
  FROM import_origins
 WHERE origin_id = $1`,
		originID,
	).Scan(
		&importFollows,
	)

	return !importFollows
}

func ImportedFollows(originID int64) {
	db, err := h.GetConnection()
	if err != nil {
		return
	}

	db.Exec(`UPDATE import_origins
   SET imported_follows = TRUE
 WHERE origin_id = $1`,
		originID,
	)
}

func ThreadComments(originID int64) bool {
	db, err := h.GetConnection()
	if err != nil {
		return false
	}

	var threadComments bool

	db.QueryRow(`SELECT imported_comments_threaded
  FROM import_origins
 WHERE origin_id = $1`,
		originID,
	).Scan(
		&threadComments,
	)

	return !threadComments
}

func ThreadedComments(originID int64) {

	db, err := h.GetConnection()
	if err != nil {
		return
	}

	db.Exec(`UPDATE import_origins
   SET imported_comments_threaded = TRUE
 WHERE origin_id = $1`,
		originID,
	)
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

// AddDeletedProfileID adds the deleted profile id to the map of imported
// profiles, helping to ensure that when 0 is looked up it will return the new
// ID. We cannot do thise via RecordImport as PostgreSQL views the zero value as
// breaking the integrity of the database.
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

	case h.ItemTypes[h.ItemTypeRole]:
		rolesLock.Lock()
		roles[oldID] = newID
		rolesLock.Unlock()

	case h.ItemTypes[h.ItemTypeWatcher]:
		watchersLock.Lock()
		watchers[oldID] = newID
		watchersLock.Unlock()

	case h.ItemTypes[h.ItemTypeHuddle]:
		huddlesLock.Lock()
		huddles[oldID] = newID
		huddlesLock.Unlock()

	case h.ItemTypes[h.ItemTypeAttachment]:
		attachmentsLock.Lock()
		attachments[oldID] = newID
		attachmentsLock.Unlock()

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
		microcosmsLock.RLock()
		if newID, ok := microcosms[oldID]; ok {
			itemID = newID
		}
		microcosmsLock.RUnlock()

	case h.ItemTypes[h.ItemTypeProfile]:
		profilesLock.RLock()
		if newID, ok := profiles[oldID]; ok {
			itemID = newID
		}
		profilesLock.RUnlock()

	case h.ItemTypes[h.ItemTypeConversation]:
		conversationsLock.RLock()
		if newID, ok := conversations[oldID]; ok {
			itemID = newID
		}
		conversationsLock.RUnlock()

	case h.ItemTypes[h.ItemTypeComment]:
		commentsLock.RLock()
		if newID, ok := comments[oldID]; ok {
			itemID = newID
		}
		commentsLock.RUnlock()

	case h.ItemTypes[h.ItemTypeWatcher]:
		rolesLock.RLock()
		if newID, ok := roles[oldID]; ok {
			itemID = newID
		}
		rolesLock.RUnlock()

	case h.ItemTypes[h.ItemTypeWatcher]:
		watchersLock.RLock()
		if newID, ok := watchers[oldID]; ok {
			itemID = newID
		}
		watchersLock.RUnlock()

	case h.ItemTypes[h.ItemTypeHuddle]:
		huddlesLock.RLock()
		if newID, ok := huddles[oldID]; ok {
			itemID = newID
		}
		huddlesLock.RUnlock()

	case h.ItemTypes[h.ItemTypeAttachment]:
		attachmentsLock.RLock()
		if newID, ok := attachments[oldID]; ok {
			itemID = newID
		}
		attachmentsLock.RUnlock()

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
