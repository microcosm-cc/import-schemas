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
	ItemTypeID      int64
	ItemID          int64
	ProfileID       int64
	Created         time.Time
	InReplyTo       sql.NullInt64
	IsVisible       bool
	IsModerated     bool
	IsDeleted       bool
	AttachmentCount int64
}

// CommentRevision struct
type CommentRevision struct {
	CommentID int64
	ProfileID int64
	Raw       string
	Created   time.Time
}

// Walk directory and load all comments into a map ID -> Filepath
// Iterate the map, loading the files into src.Comments
//

type Node struct {
	Comment  *src.Comment
	Replies  []*Node
	ReplyIds []int64
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

	loadedCommentMap := make(map[int64]Node)
	roots := []int64{}
	replies := []int64{}

	commentIDs := files.GetIDs(args.ItemTypeID)

	for _, ID := range commentIDs {
		srcComment := src.Comment{}
		// errcheck
		files.JSONFileToInterface(
			files.GetPath(args.ItemTypeID, ID),
			&srcComment,
		)
		loadedCommentMap[ID] = Node{
			Comment: &srcComment,
		}
		if srcComment.InReplyTo > 0 {
			replies = append(replies, srcComment.ID)
		} else {
			roots = append(roots, srcComment.ID)
		}
	}
	fmt.Printf("Roots: %d, Replies: %d\n", roots, replies)

	// For each reply, find the parent node and append self to the replies slice.
	for _, ID := range replies {
		inReplyTo := loadedCommentMap[ID].Comment.InReplyTo
		// Append this ID to the list of replies of the "parent"
		// Only if it's not a merged thread.
		if ID > inReplyTo {
			// Can we use a pointer to the map value instead of putting again?
			parent := loadedCommentMap[inReplyTo]
			parent.ReplyIDs = append(parent.Replies, ID)
            parent.Replies = append(...)
			loadedCommentMap[inReplyTo] = parent
		} else {
			fmt.Printf("Broke a cycle, comment: %d, parent: %d\n", ID, inReplyTo)
		}
	}

	// Adapted from conc/conc.go
	bar := pb.StartNew(len(commentIDs))
	done := make(chan struct{})
	quit := false
	tasks := make(chan in64, len(roots))

	// Now, import all roots, then traverse the roots BFS.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
		}()
	}

}

// Return a list of comments to import.
// There are no cycles, so we don't need to check if the current
// node has already been visited.
func bfs(node Node) []int64 {
	visited = []int64{}
	queue := []Node{node}

	for n := range queue {
		visited = append(visited, node.Replies...)
		queue = append(queue, node.Replies...)
	}
	return visited
}

func importComment(args conc.Args, itemID int64, srcComment src.Comment) error {

	// Skip when it already exists
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping comment %d", itemID)
		}
		return nil
	}

	if srcComment == nil {
		srcComment = src.Comment{}
		err := files.JSONFileToInterface(
			files.GetPath(args.ItemTypeID, itemID),
			&srcComment,
		)
		if err != nil {
			glog.Errorf("Failed to load comment from JSON: %+v", err)
			return err
		}
	}

	createdByID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeComment],
		srcComment.Author,
	)
	if createdByID == 0 {
		createdByID = args.DeletedProfileID
		if glog.V(2) {
			glog.Infof(
				"Using deleted profile for profile ID %d",
				srcComment.Author,
			)
		}
	}

	// Determine which new conversation ID this comment belongs to.
	conversationID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeConversation],
		srcComment.Association.OnID,
	)
	if conversationID == 0 {
		return fmt.Errorf(
			"Exported conversation ID %d does not have an imported ID, "+
				"skipped comment %d\n",
			srcComment.ID,
			itemID,
		)
	}

	// The comment this comment replies to may have been imported previously.
	if srcComment.InReplyTo > 0 {
		replyToID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeComment],
			srcComment.InReplyTo,
		)
		if replyToID > 0 {
			srcComment.InReplyTo = replyToID
		} else {
			// Log that InReplyTo wasn't found.
			if glog.V(2) {
				glog.Infof(
					"InReplyTo for comment ID %d does not have an imported ID",
					itemID,
				)
			}
		}
	}

	visible := !srcComment.Deleted && !srcComment.Moderated

	// InReplyTo is NULL if 0 or higher than the current comment's ID (indicating
	// a merge or some other modification to the original thread).
	var inReplyTo sql.NullInt64
	if srcComment.InReplyTo > 0 && srcComment.InReplyTo < srcComment.ID {
		inReplyTo = sql.NullInt64{Valid: true, Int64: srcComment.InReplyTo}
	}

	comment := Comment{
		ItemTypeID:      h.ItemTypes[h.ItemTypeConversation],
		ItemID:          conversationID,
		ProfileID:       createdByID,
		Created:         srcComment.DateCreated,
		InReplyTo:       inReplyTo,
		IsVisible:       visible,
		IsModerated:     srcComment.Moderated,
		IsDeleted:       srcComment.Deleted,
		AttachmentCount: 0,
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to createComment for CommentID %d: %+v", itemID, err)
		return err
	}
	defer tx.Rollback()

	iCID, err := createComment(tx, comment)
	if err != nil {
		glog.Errorf("Failed to createComment for conversation %d: %+v", itemID, err)
		return err
	}

	// Fetch new ID of revision creator.
	revisionAuthorID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcComment.Versions[0].Editor,
	)
	// Fall back to original comment author.
	if revisionAuthorID == 0 {
		revisionAuthorID = createdByID
	}

	revision := CommentRevision{
		CommentID: iCID,
		ProfileID: revisionAuthorID,
		Raw:       srcComment.Versions[0].Text,
		Created:   srcComment.Versions[0].DateModified,
	}

	_, err = createRevision(tx, revision)
	if err != nil {
		glog.Errorf("Failed to createRevision for CommentID: %d", srcComment.ID)
		return err
	}

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcComment.ID,
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

// createComment puts a single comment into the database.
func createComment(tx *sql.Tx, c Comment) (cID int64, err error) {
	err = tx.QueryRow(
		`INSERT INTO comments (
            item_type_id, item_id, profile_id, created, is_visible,
            is_moderated, is_deleted, in_reply_to, attachment_count, yay_count,
            meh_count, grr_count
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, 0, 0,
            0, 0
        ) RETURNING comment_id;`,
		c.ItemTypeID,
		c.ItemID,
		c.ProfileID,
		c.Created,
		c.IsVisible,
		c.IsModerated,
		c.IsDeleted,
		c.InReplyTo,
	).Scan(&cID)
	return
}

// createRevision puts a single comment into the database.
func createRevision(tx *sql.Tx, r CommentRevision) (rID int64, err error) {
	err = tx.QueryRow(
		`INSERT INTO revisions (
            comment_id,
            profile_id,
            "raw",
            created,
            is_current
        ) VALUES (
            $1,
            $2,
            $3,
            $4,
            true
        ) RETURNING revision_id`,
		r.CommentID,
		r.ProfileID,
		r.Raw,
		r.Created,
	).Scan(&rID)
	return
}
