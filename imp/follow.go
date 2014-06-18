package imp

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"strings"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

func importFollows(args conc.Args, gophers int) (errors []error) {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeWatcher]

	fmt.Println("Importing follows...")
	glog.Info("Importing follows...")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, errors)
	}

	return conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importFollow,
		gophers,
	)

}

// For each follow file, map the old profile ID (and following profile ID) to new ones.
// If there are any conversation follows, map to new ID.
// Notify: true sets the email field to true. Ignore SMS.
// Follow is one follow record, this implies multiple watchers.
func importFollow(args conc.Args, itemID int64) error {

	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		glog.Infof("Skipping watchers for profile %d", itemID)
		return nil
	}

	srcFollow := src.Follow{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&srcFollow,
	)
	if err != nil {
		glog.Errorf("Failed to load follow from JSON: %+v", err)
		return err
	}

	// New owner profile ID
	PID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcFollow.Author,
	)
	if PID == 0 {
		err = errors.New(fmt.Sprintf("No new ID for profile %d, skipped follows", srcFollow.Author))
		glog.Error(err.Error())
		return err
	}

	// Users following
	for _, user := range srcFollow.Users {
		// Fetch the new profile ID of user being followed.
		followPID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeProfile],
			user.ID,
		)
		w := models.WatcherType{
			ProfileID:  PID,
			ItemID:     followPID,
			ItemTypeID: h.ItemTypes[h.ItemTypeProfile],
			SendEmail:  user.Notify,
		}
		_, err := w.Import()
		if err != nil {
			// Ignore complaints about non-uniques
			if !strings.Contains(err.Error(), "unique") {
				glog.Error(err.Error())
			}
		}
	}

	// Forums following
	for _, forum := range srcFollow.Forums {
		FID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeMicrocosm],
			forum.ID,
		)
		w := models.WatcherType{
			ProfileID:  PID,
			ItemID:     FID,
			ItemTypeID: h.ItemTypes[h.ItemTypeMicrocosm],
			SendEmail:  forum.Notify,
		}
		_, err := w.Import()
		if err != nil {
			// Ignore complaints about non-uniques
			if !strings.Contains(err.Error(), "unique") {
				glog.Error(err.Error())
			}
		}
	}

	// Conversations following
	for _, conv := range srcFollow.Conversations {
		CID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeConversation],
			conv.ID,
		)
		w := models.WatcherType{
			ProfileID:  PID,
			ItemID:     CID,
			ItemTypeID: h.ItemTypes[h.ItemTypeConversation],
			SendEmail:  conv.Notify,
		}
		_, err := w.Import()
		if err != nil {
			// Ignore complaints about non-uniques
			if !strings.Contains(err.Error(), "unique") {
				glog.Error(err.Error())
			}
		}
	}

	// One follow ID will map to multiple watcher IDs.
	// Store a '1' for the new follow ID value, to signify
	// that all follows for the current profile have been processed.
	tx, err := h.GetTransaction()
	if err != nil {
		glog.Error(err.Error())
		return err
	}
	err = accounting.RecordImport(tx, args.OriginID, args.ItemTypeID, itemID, 1)
	if err != nil {
		glog.Error(err.Error())
	}
	err = tx.Commit()
	if err != nil {
		glog.Error(err.Error())
	}
	return err
}
