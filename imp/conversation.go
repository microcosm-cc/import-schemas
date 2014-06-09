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

// Conversation struct
type Conversation struct {
	ConversationID int64
	MicrocosmID    int64
	Title          string
	Created        time.Time
	CreatedBy      int64
	Edited         time.Time
	EditedBy       int64
	EditReason     string
	IsSticky       bool
	IsOpen         bool
	IsDeleted      bool
	IsModerated    bool
	IsVisible      bool
	CommentCount   int64
	ViewCount      int64
}

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

	c := Conversation{
		MicrocosmID: microcosmID,
		Title:       srcConversation.Name,
		Created:     srcConversation.DateCreated,
		CreatedBy:   createdByID,
		ViewCount:   srcConversation.ViewCount,
		IsSticky:    false,
		IsOpen:      true,
		IsDeleted:   false,
		IsModerated: false,
		IsVisible:   true,
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to createMicrocosm for forum %d: %+v", itemID, err)
		return err
	}
	defer tx.Rollback()

	iCID, err := createConversation(tx, c)
	if err != nil {
		glog.Errorf(
			"Failed to createConversation for conversation %d: %+v",
			itemID,
			err,
		)
		return err
	}

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcConversation.ID,
		iCID,
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

// StoreConversation puts a conversation into the database
func createConversation(tx *sql.Tx, c Conversation) (cID int64, err error) {

	err = tx.QueryRow(`
INSERT INTO conversations (
    microcosm_id, title, created, created_by, is_sticky,
    is_open, is_deleted, is_moderated, is_visible
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9
) RETURNING conversation_id;`,
		c.MicrocosmID,
		c.Title,
		c.Created,
		c.CreatedBy,
		c.IsSticky,

		c.IsOpen,
		c.IsDeleted,
		c.IsModerated,
		c.IsVisible,
	).Scan(
		&cID,
	)

	return
}
