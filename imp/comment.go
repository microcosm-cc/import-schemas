package imp

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/cheggaaa/pb"
	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

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

	// Map of old comment ID to pointer of loaded comment and IDs of replies.
	loadedNodeMap := make(map[int64]*Node)
	// Roots are comments where InReplyTo is 0.
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
			glog.Errorf("Failed to unmarshal comment %d: %s\n", ID, err)
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
				// Parent not found, so treat this comment as a root.
				glog.Infof("Could not find InReplyTo: %d for comment %d\n", srcComment.InReplyTo, ID)
				roots = append(roots, ID)
			}
		}
	}
	bar.Finish()

	glog.Info("Found %d comments without parents\n", len(roots))

	// Iterate the roots, storing each root first, then doing breadth-first traversal and storing all replies.
	taskBar := pb.StartNew(len(roots))
	var wg sync.WaitGroup
	tasks := make(chan int64, len(roots)+1)

	for i := 0; i < 50; i++ {
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
			wg.Done()
		}()
	}

	// Send all root IDs to the tasks channel.
	for _, id := range roots {
		tasks <- id
	}
	close(tasks)
	glog.Info("Waiting for comment tasks to finish\n")
	wg.Wait()
	taskBar.Finish()

	return errors
}

// Traverse replies breadth-first and store them.
func bfs(args conc.Args, node *Node) {
	queue := []*Node{node}
	for _, n := range queue {
		err := importComment(args, *n.Comment)
		if err != nil {
			fmt.Print(err)
			glog.Errorf("Error importing comment: %s\n", err)
		}
		queue = append(queue, n.Replies...)
	}
}

func importComment(args conc.Args, srcComment src.Comment) error {

	// Skip if comment already imported.
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, srcComment.ID) > 0 {
		glog.Infof("Skipping already-imported comment %d\n", srcComment.ID)
		return nil
	}

	// Fetch new profile ID of comment author.
	createdByID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeComment],
		srcComment.Author,
	)
	if createdByID == 0 {
		createdByID = args.DeletedProfileID
		glog.Infof("Using deleted profile for profile ID %d\n", srcComment.Author)
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
			srcComment.Association.OnID,
			srcComment.ID,
		)
	}

	// The comment this comment replies to should have been imported previously.
	if srcComment.InReplyTo > 0 {
		replyToID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeComment],
			srcComment.InReplyTo,
		)
		if replyToID > 0 {
			srcComment.InReplyTo = replyToID
		} else {
			glog.Infof(
				"InReplyTo comment ID %d (needed by comment %d) does not have an imported ID\n",
				srcComment.InReplyTo,
				srcComment.ID,
			)
		}
	}

	visible := !srcComment.Deleted && !srcComment.Moderated

	// InReplyTo is NULL if the ID is greater than the current comment's ID (indicating
	// a merge or some other modification to the original thread).
	var inReplyTo sql.NullInt64
	if srcComment.InReplyTo > srcComment.ID {
		inReplyTo = sql.NullInt64{Valid: true, Int64: srcComment.InReplyTo}
	}

	comment := models.CommentSummaryType{
		ItemType:          "conversation",
		ItemId:            conversationID,
		InReplyToNullable: inReplyTo,
		Markdown:          srcComment.Versions[0].Text,
	}
	comment.Meta.CreatedById = createdByID
	comment.Meta.Created = srcComment.DateCreated
	comment.Meta.Flags.Visible = visible
	comment.Meta.Flags.Moderated = srcComment.Moderated
	comment.Meta.Flags.Deleted = srcComment.Deleted

	// Import creates and commits/rollbacks its own transaction.
	_, err := comment.Import(args.SiteID)
	if err != nil || comment.Id < 1 {
		glog.Errorf("Failed to import comment for conversation %d: %s", srcComment.ID, err)
		return err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcComment.ID,
		comment.Id,
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

	return nil
}
