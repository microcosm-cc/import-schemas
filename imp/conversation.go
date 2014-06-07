package imp

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/cheggaaa/pb"

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
func importConversations(args conc.Args) (errors []error) {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeConversation]

	log.Print("Importing conversations...")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, errors)
		return
	}

	ids := files.GetIDs(args.ItemTypeID)

	bar := pb.StartNew(len(ids))

	// Iterate conversations in order.
	for _, id := range ids {
		err := importConversation(args, id)
		if err != nil {
			errors = append(errors, err)
		}
		bar.Increment()
	}
	bar.Finish()
	return
}

// importConversation imports a single conversation or skips it if it has been
// done aleady
func importConversation(args conc.Args, itemID int64) error {
	srcConversation := src.Conversation{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		srcConversation,
	)
	if err != nil {
		return err
	}

	// Look up the author profile based on the old user ID.
	authorI := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcConversation.Author,
	)
	if authorI == 0 {
		return fmt.Errorf(
			"Exported user ID %d does not have an imported profile, "+
				"skipped conversation %d\n",
			srcConversation.Author,
			itemID,
		)
	}

	MID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeMicrocosm],
		srcConversation.ForumID,
	)
	if MID == 0 {
		return fmt.Errorf(
			"Exported forum ID %d does not have an imported microcosm, "+
				"skipped conversation %d\n",
			srcConversation.ForumID,
			itemID,
		)
	}

	c := Conversation{
		MicrocosmID: MID,
		Title:       srcConversation.Name,
		Created:     srcConversation.DateCreated,
		CreatedBy:   authorI,
		ViewCount:   srcConversation.ViewCount,
		IsSticky:    false,
		IsOpen:      true,
		IsDeleted:   false,
		IsModerated: false,
		IsVisible:   true,
	}

	tx, err := h.GetTransaction()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	iCID, err := createConversation(tx, c)
	if err != nil {
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
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
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
