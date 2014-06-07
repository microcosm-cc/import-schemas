package imp

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/cheggaaa/pb"

	exports "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
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

// ImportConversations walks the tree importing each conversation
func ImportConversations(
	rootpath string,
	siteID int64,
	originID int64,
) (
	errors []error,
) {

	var itemTypeID = h.ItemTypes[h.ItemTypeConversation]

	log.Print("Importing conversations...")

	err := files.WalkExportTree(rootpath, itemTypeID)
	if err != nil {
		exitWithError(err, errors)
		return
	}

	ids := files.GetIDs(itemTypeID)

	bar := pb.StartNew(len(ids))

	// Iterate conversations in order.
	for _, CID := range ids {

		bar.Increment()

		eConv := exports.Conversation{}
		err = files.JSONFileToInterface(files.GetPath(itemTypeID, CID), eConv)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// Look up the author profile based on the old user ID.
		authorI := accounting.GetNewID(originID, h.ItemTypes[h.ItemTypeProfile], eConv.Author)
		if authorI == 0 {
			errors = append(errors, fmt.Errorf(
				"Exported user ID %d does not have an imported profile, skipped conversation %d\n",
				eConv.Author,
				CID,
			))
			continue
		}

		MID := accounting.GetNewID(originID, h.ItemTypes[h.ItemTypeMicrocosm], eConv.ForumID)
		if MID == 0 {
			errors = append(errors, fmt.Errorf(
				"Exported forum ID %d does not have an imported microcosm, skipped conversation %d\n",
				eConv.ForumID,
				CID,
			))
		}

		c := Conversation{
			MicrocosmID: MID,
			Title:       eConv.Name,
			Created:     eConv.DateCreated,
			CreatedBy:   authorI,
			ViewCount:   eConv.ViewCount,
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

		iCID, err := StoreConversation(tx, c)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		err = accounting.RecordImport(
			tx,
			originID,
			h.ItemTypes[h.ItemTypeConversation],
			eConv.ID,
			iCID,
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
	return
}

// StoreConversation puts a conversation into the database
func StoreConversation(tx *sql.Tx, c Conversation) (cID int64, err error) {

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
