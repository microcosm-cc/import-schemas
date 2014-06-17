package imp

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

// importMicrocosms iterates a the export directory, storing each forums
// individually
func importMicrocosms(args conc.Args, gophers int) (errors []error) {

	// Forums
	args.ItemTypeID = h.ItemTypes[h.ItemTypeMicrocosm]

	fmt.Println("Importing forums as microcosms...")
	glog.Info("Importing forums as microcosms...")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, errors)
	}

	return conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importMicrocosm,
		gophers,
	)
}

func importMicrocosm(args conc.Args, itemID int64) error {

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
	m := models.MicrocosmType{}
	m.SiteId = args.SiteID
	m.Title = srcForum.Name
	m.OwnedById = createdByID
	m.Description = srcForum.Text

	m.Meta.Created = time.Now()
	m.Meta.CreatedBy = createdByID

	m.Meta.Flags.Open = srcForum.Open
	m.Meta.Flags.Sticky = srcForum.Sticky
	m.Meta.Flags.Moderated = srcForum.Moderated
	m.Meta.Flags.Deleted = srcForum.Deleted
	m.Meta.Flags.Visible = true

	_, err = m.Import()
	if err != nil {
		glog.Errorf(
			"Failed to create microcosm for microcosm %d: %+v",
			srcForum.ID,
			err,
		)
		return err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to get transaction: %+v", err)
		return err
	}
	defer tx.Rollback()

	if err != nil {
		glog.Errorf("Failed to createMicrocosm for forum %d: %+v", itemID, err)
		return err
	}
	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcForum.ID,
		m.Id,
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
