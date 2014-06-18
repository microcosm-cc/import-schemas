package imp

import (
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	"github.com/microcosm-cc/microcosm/audit"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

var (
	// map[oldCommentId]oldInReplyToId
	commentReplies     = make(map[int64]int64)
	commentRepliesLock = sync.Mutex{}

	// a slice of oldCommentIds that we have imported *on this run*
	commentsImportedThisRun = []int64{}
)

func importComments(args conc.Args, gophers int) []error {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeComment]

	fmt.Println("Walking comment tree...")
	glog.Info("Walking comment tree...")
	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, []error{})
	}

	// Import comments
	fmt.Println("Importing comments...")
	glog.Info("Importing comments...")
	errs := conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importComment,
		gophers,
	)

	// Process replies
	fmt.Println("Threading comments...")
	glog.Info("Threading comments...")
	errs2 := conc.RunTasks(
		commentsImportedThisRun,
		args,
		importCommentInReplyTo,
		gophers,
	)
	errs = append(errs, errs2...)

	return errs
}

func importComment(args conc.Args, itemID int64) error {

	// Skip if comment already imported.
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping comment %d\n", itemID)
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

	// Fetch new profile ID of comment author.
	createdByID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcComment.Author,
	)
	if createdByID == 0 {
		createdByID = args.DeletedProfileID
		if glog.V(2) {
			glog.Infof(
				"Using deleted profile for profile ID %d\n",
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
			srcComment.Association.OnID,
			srcComment.ID,
		)
	}

	// Store the inReplyTo for post-processing
	if srcComment.InReplyTo > 0 {
		commentRepliesLock.Lock()
		commentReplies[srcComment.ID] = srcComment.InReplyTo
		commentRepliesLock.Unlock()
	}

	m := models.CommentSummaryType{}
	m.ItemType = "conversation"
	m.ItemId = conversationID
	m.Markdown = srcComment.Versions[0].Text

	m.Meta.Created = srcComment.DateCreated
	m.Meta.CreatedById = createdByID

	m.Meta.Flags.Deleted = srcComment.Deleted
	m.Meta.Flags.Moderated = srcComment.Moderated
	m.Meta.Flags.Visible = !srcComment.Deleted && !srcComment.Moderated

	// Import creates and commits/rollbacks its own transaction.
	_, err = m.Import(args.SiteID)
	if err != nil || m.Id < 1 {
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

	// Log the IP address
	audit.Create(
		args.SiteID,
		h.ItemTypes[h.ItemTypeComment],
		m.Id,
		createdByID,
		srcComment.DateCreated,
		net.ParseIP(srcComment.IPAddress),
	)

	commentsImportedThisRun = append(commentsImportedThisRun, srcComment.ID)

	if glog.V(2) {
		glog.Infof("Successfully imported comment %d", srcComment.ID)
	}

	return nil
}

func importCommentInReplyTo(args conc.Args, itemID int64) error {

	if inReplyTo, ok := commentReplies[itemID]; ok {
		commentID := accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID)
		replyToID := accounting.GetNewID(args.OriginID, args.ItemTypeID, inReplyTo)

		if commentID > 0 && replyToID > 0 {
			return models.SetCommentInReplyTo(args.SiteID, commentID, replyToID)
		}
	}

	return nil
}
