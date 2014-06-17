package imp

import (
	"fmt"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

// importConversations walks the tree importing each conversation
func importConversations(args conc.Args, gophers int) (errors []error) {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeConversation]

	fmt.Println("Importing conversations...")
	glog.Info("Importing conversations...")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, errors)
	}

	return conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importConversation,
		gophers,
	)
}

// importConversation imports a single conversation or skips it if it has been
// done aleady
func importConversation(args conc.Args, itemID int64) error {

	// Skip when it already exists
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping conversation %d", itemID)
		}
		return nil
	}

	srcConversation := src.Conversation{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&srcConversation,
	)
	if err != nil {
		glog.Errorf("Failed to load conversation from JSON: %+v", err)
		return err
	}

	// Look up the author profile based on the old user ID.
	createdByID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcConversation.Author,
	)
	if createdByID == 0 {
		createdByID = args.DeletedProfileID
		if glog.V(2) {
			glog.Infof(
				"Using deleted profile for profile ID %d",
				srcConversation.Author,
			)
		}
	}

	microcosmID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeMicrocosm],
		srcConversation.ForumID,
	)
	if microcosmID == 0 {
		return fmt.Errorf(
			"Exported forum ID %d does not have an imported microcosm, "+
				"skipped conversation %d\n",
			srcConversation.ForumID,
			itemID,
		)
	}

	if len(srcConversation.Name) > 150 {
		srcConversation.Name = srcConversation.Name[:150]
	}

	m := models.ConversationType{}
	m.MicrocosmId = microcosmID
	m.Title = srcConversation.Name
	m.ViewCount = srcConversation.ViewCount
	m.Meta.Created = srcConversation.DateCreated
	m.Meta.CreatedById = createdByID
	m.Meta.Flags.Deleted = srcConversation.Deleted
	m.Meta.Flags.Moderated = srcConversation.Moderated
	m.Meta.Flags.Open = srcConversation.Open
	m.Meta.Flags.Sticky = srcConversation.Sticky

	_, err = m.Import(args.SiteID, createdByID)
	if err != nil {
		glog.Errorf(
			"Failed to createConversation for conversation %d: %+v",
			itemID,
			err,
		)
		return err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to createMicrocosm for forum %d: %+v", itemID, err)
		return err
	}
	defer tx.Rollback()

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcConversation.ID,
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
		glog.Infof("Successfully imported conversation %d", itemID)
	}
	return nil
}
