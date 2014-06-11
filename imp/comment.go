package imp

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/cheggaaa/pb"
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
	Comment *src.Comment
	Replies []*Node
}

func importComments(args conc.Args) (errors []error) {

	// Comments.
	args.ItemTypeID = h.ItemTypes[h.ItemTypeComment]

	glog.Info("Loading comments...")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, errors)
	}

	loadedNodeMap := make(map[int64]*Node)
	roots := []int64{}

	commentIDs := files.GetIDs(args.ItemTypeID)
	bar := pb.StartNew(len(commentIDs))

	for _, ID := range commentIDs {
		bar.Increment()

		srcComment := src.Comment{}
		err = files.JSONFileToInterface(
			files.GetPath(args.ItemTypeID, ID),
			&srcComment,
		)
		if err != nil {
			glog.Errorf("Failed to unmarshal comment %d: %s", ID, err)
			continue
		}

		// Create a Node representing this comment.
		node := Node{
			Comment: &srcComment,
		}
		loadedNodeMap[ID] = &node

		// If the comment is a "root", append to the list.
		if srcComment.InReplyTo == 0 {
			roots = append(roots, ID)
		} else if srcComment.InReplyTo > 0 {
			// Find the parent comment and append this node to the list of replies.
			parent, ok := loadedNodeMap[srcComment.InReplyTo]
			if ok {
				parent.Replies = append(parent.Replies, &node)
			} else {
				glog.Errorf("Could not find InReplyTo: %d for comment %d\n", srcComment.InReplyTo, ID)
			}
		}
	}
	bar.Finish()

	fmt.Printf("Found %d roots\n", len(roots))
	glog.Info("Found %d roots\n", len(roots))

	// Iterate the roots, storing each root first, then doing breadth-first traversal and storing all replies.
	taskBar := pb.StartNew(len(roots))
	var wg sync.WaitGroup
	tasks := make(chan int64, len(roots)+1)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			for ID := range tasks {
				node, ok := loadedNodeMap[ID]
				if ok {
					bfs(args, node)
				} else {
					glog.Errorf("Could not retrieve root: %d\n", ID)
				}
				taskBar.Increment()
			}
		}()
	}

	// Send all root IDs to the tasks channel.
	for _, id := range roots {
		tasks <- id
	}
	close(tasks)
	wg.Wait()
	taskBar.Finish()

	return errors
}

// Traverse replies breadth-first and store them.
func bfs(args conc.Args, node *Node) {
	queue := []*Node{node}
	for _, n := range queue {
		err := importComment(args, n.Comment.ID, *n.Comment)
		if err != nil {
			fmt.Print(err)
			glog.Error(err)
		}
		queue = append(queue, n.Replies...)
	}
}

func importComment(args conc.Args, itemID int64, srcComment src.Comment) error {

	// Skip when it already exists
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping comment %d", itemID)
		}
		return nil
	}

	if srcComment.ID == 0 {
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
