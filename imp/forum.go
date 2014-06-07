package imp

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

// Microcosm struct
type Microcosm struct {
	Title       string
	Description string
	SiteID      int64
	Created     time.Time
	CreatedBy   int64
	OwnedBy     int64
	IsSticky    bool
	IsModerated bool
	IsOpen      bool
	IsDeleted   bool
	IsVisible   bool
}

// importForums iterates a the export directory, storing each forums
// individually
func importForums(args conc.Args, gophers int) (errors []error) {

	// Forums
	args.ItemTypeID = h.ItemTypes[h.ItemTypeMicrocosm]

	fmt.Println("Importing forums...")
	glog.Info("Importing forums...")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, errors)
		return
	}

	return conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importForum,
		gophers,
	)
}

func importForum(args conc.Args, itemID int64) error {

	// Skip when it already exists
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping forum %d", itemID)
		}
		return nil
	}

	srcForum := src.Forum{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&srcForum,
	)
	if err != nil {
		glog.Errorf("Failed to load forum from JSON: %+v", err)
		return err
	}

	createdByID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcForum.Author,
	)
	if createdByID == 0 {
		return fmt.Errorf(
			`Cannot find existing user for src author %d for src forum %d`,
			srcForum.Author,
			srcForum.ID,
		)
	}

	// CreatedBy and OwnedBy are assumed to be the site owner.
	m := Microcosm{
		SiteID:      args.SiteID,
		Title:       srcForum.Name,
		Description: srcForum.Text,
		Created:     time.Now(),
		CreatedBy:   createdByID,
		OwnedBy:     createdByID,
		IsOpen:      srcForum.Open,
		IsSticky:    srcForum.Sticky,
		IsModerated: srcForum.Moderated,
		IsDeleted:   srcForum.Deleted,
		IsVisible:   true,
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to get transaction: %+v", err)
		return err
	}
	defer tx.Rollback()

	MID, err := createMicrocosm(tx, m)
	if err != nil {
		glog.Errorf("Failed to createMicrocosm for forum %d: %+v", itemID, err)
		return err
	}
	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcForum.ID,
		MID,
	)
	if err != nil {
		glog.Errorf("Failed to recordImport: %+v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		glog.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	if glog.V(2) {
		glog.Infof("Successfully imported forum %d", itemID)
	}
	return nil
}

// createMicrocosm puts an individual microcosm into the database
func createMicrocosm(tx *sql.Tx, m Microcosm) (int64, error) {

	var microcosmID int64

	err := tx.QueryRow(`
INSERT INTO microcosms (
    title, description, site_id, created, created_by, owned_by,
    is_sticky, is_moderated, is_open, is_deleted, is_visible
) VALUES (
    $1, $2, $3, NOW(), $4, $5,
    $6, $7, $8, $9, $10
) RETURNING microcosm_id;`,
		m.Title,
		m.Description,
		m.SiteID,
		m.CreatedBy,
		m.OwnedBy,

		m.IsSticky,
		m.IsModerated,
		m.IsOpen,
		m.IsDeleted,
		m.IsVisible,
	).Scan(
		&microcosmID,
	)

	return microcosmID, err
}
