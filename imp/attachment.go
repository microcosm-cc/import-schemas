package imp

import (
	"fmt"
	"net"
	"strings"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	"github.com/microcosm-cc/microcosm/audit"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

func importAttachments(args conc.Args, gophers int) (errors []error) {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeAttachment]

	fmt.Println("Loading attachments")
	glog.Info("Loading attachments")

	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, []error{})
	}

	return conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importHuddle,
		gophers,
	)
}

func importAttachment(args conc.Args, itemID int64) error {

	// Attachment new ID is the PK from the attachments table.
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping attachment %d\n", itemID)
		}
		return nil
	}

	srcAttach := src.Attachment{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&srcAttach,
	)
	if err != nil {
		glog.Errorf("Failed to load attachment from JSON: %+v", err)
		return err
	}

	// Use FileMetadata.Import which idempotently creates necessary row.
	fm := models.FileMetadata{
		Created:  srcAttach.DateCreated,
		FileName: srcAttach.Name,
		FileSize: srcAttach.ContentSize,
		MimeType: srcAttach.MimeType,
		Width:    srcAttach.Width,
		Height:   srcAttach.Height,
	}
	// TODO: read attachment content-url, populate fd.Content and generate SHA-1.
	fd.Import()

	// Look up the author profile based on the old user ID.
	authorID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcAttach.Author,
	)

	if authorID == 0 {
		authorID = args.DeletedProfileID
		if glog.V(2) {
			glog.Infof(
				"Using deleted profile for profile ID %d",
				srcAttach.Author,
			)
		}
	}

	at := models.AttachmentType{
		ProfileId:        authorID,
		AttachmentMetaId: fm.Id,
		// TODO: determine the association type.
		ItemTypeId: h.ItemTypes[h.ItemTypeComment],
		// TODO: handle multiple assocations.
		ItemId:   srcAttach.Associations[0].OnID,
		FileHash: "SHA-1",
		FileName: srcAttach.Name,
		FileExt:  "...",
		Created:  srcAttach.DateCreated,
	}
	at.Import()

	tx, err := h.GetTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcMessage.ID,
		at.Id,
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

}
