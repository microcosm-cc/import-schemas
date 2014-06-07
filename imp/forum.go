package imp

import (
	"database/sql"
	"log"
	"time"

	"github.com/cheggaaa/pb"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
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

// StoreMicrocosm puts an individual microcosm into the database
func StoreMicrocosm(tx *sql.Tx, m Microcosm) (int64, error) {

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

// ImportForums iterates a the export directory, storing each forums
// individually
func ImportForums(
	rootpath string,
	siteID int64,
	adminID int64,
	originID int64,
) (
	errors []error,
) {

	// Forums
	var itemTypeID = h.ItemTypes[h.ItemTypeMicrocosm]

	log.Print("Importing forums...")

	err := files.WalkExportTree(rootpath, itemTypeID)
	if err != nil {
		exitWithError(err, errors)
		return
	}

	ids := files.GetIDs(itemTypeID)

	bar := pb.StartNew(len(ids))

	for _, FID := range ids {

		bar.Increment()

		eForum := src.Forum{}
		err = files.JSONFileToInterface(files.GetPath(itemTypeID, FID), eForum)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		tx, err := h.GetTransaction()
		if err != nil {
			return
		}
		defer tx.Rollback()

		// CreatedBy and OwnedBy are assumed to be the site owner.
		m := Microcosm{
			SiteID:      siteID,
			Title:       eForum.Name,
			Description: eForum.Text,
			Created:     time.Now(),
			CreatedBy:   adminID,
			OwnedBy:     adminID,
			IsOpen:      eForum.Open,
			IsSticky:    eForum.Sticky,
			IsModerated: eForum.Moderated,
			IsDeleted:   eForum.Deleted,
			IsVisible:   true,
		}
		MID, err := StoreMicrocosm(tx, m)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		err = accounting.RecordImport(
			tx,
			originID,
			h.ItemTypes[h.ItemTypeMicrocosm],
			eForum.ID,
			MID,
		)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		err = tx.Commit()
		if err != nil {
			errors = append(errors, err)
			continue
		}
	}

	bar.Finish()

	return errors
}
