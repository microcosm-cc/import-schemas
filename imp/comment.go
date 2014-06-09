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

// Comment struct
type Comment struct {
	CommentID       int64
	ItemTypeID      int64
	ItemID          int64
	ProfileID       int64
	Created         time.Time
	InReplyTo       int64
	IsVisible       bool
	IsModerated     bool
	IsDeleted       bool
	AttachmentCount int64
}

// CommentRevision struct
type CommentRevision struct {
	RevisionID int64
	CommentID  int64
	ProfileID  int64
	Raw        string
	HTML       string
	Created    time.Time
	IsCurrent  bool
}

func importComments(args conc.Args) (errors []error) {

	// Comments.
	args.ItemTypeID = h.ItemTypes[h.ItemTypeComment]

	fmt.Println("Importing comments...")
	glog.Info("Importing comments...")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, errors)
	}

	return conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importComment,
		1,
	)
}

func importComment(args conc.Args, itemID int64) error {

	// Skip when it already exists
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping comment %d", itemID)
		}
		return nil
	}

	srcComment := src.Comment{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&srcComment,
	)
	if err != nil {
		glog.Errorf("Failed to load comment from JSON: %+v", err)
		return err
	}

	createdByID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeComment],
		srcComment.Author,
	)
	if createdByID == 0 {
		// Owner of this comment may have been deleted. How can we tell?
		return fmt.Errorf(
			`Cannot find existing user for src author %d for src comment %d`,
			srcComment.Author,
			srcComment.ID,
		)
	}
	return nil
}

// createComment puts a single comment into the database.
// TODO: comment revision
func StoreComment(tx *sql.Tx, c Comment) (int64, error) {

	var cID int64
	err := tx.QueryRow(
		`INSERT INTO comments () VALUES () RETURNING conversation_id;`,
	).Scan(&cID)

	if err != nil {
		return cID, err
	}
	err = tx.Commit()
	return cID, err

}
