package imp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

func importAttachments(args conc.Args, gophers int) (errors []error) {

	// Doesn't exist?
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
		importAttachment,
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
	fm := models.FileMetadataType{
		Created:  srcAttach.DateCreated,
		FileName: srcAttach.Name,
		FileSize: srcAttach.ContentSize,
		MimeType: srcAttach.MimeType,
		Width:    srcAttach.Width,
		Height:   srcAttach.Height,
	}

	parts := strings.SplitN(srcAttach.ContentURL, ",", 2)
	if len(parts) != 2 {
		err = errors.New(fmt.Sprintf("Unexpected data URI, attachment %d\n", srcAttach.ID))
		glog.Error(err)
		return err
	}

	content, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		err = fmt.Errorf("Could not decode attachment %d: %s\n", srcAttach.ID, err)
		glog.Error(err)
		return err
	}
	fm.Content = content

	SHA1, err := h.Sha1(content)
	if err != nil {
		err = fmt.Errorf("Could not generate SHA-1 for attachment %d: %s\n", srcAttach.ID, err)
		glog.Error(err)
		return err
	}
	fm.FileHash = SHA1

	max := int64(1)<<32 - 1
	_, err = fm.Import(max, max)
	if err != nil {
		glog.Error(err)
		return err
	}

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

	for _, assoc := range srcAttach.Associations {

		var assocItemTypeID int64
		switch assoc.OnType {
		case "comment":
			assocItemTypeID = 4
		case "profile":
			assocItemTypeID = 3
		default:
			return fmt.Errorf("Unknown attachment association: %s\n", assoc.OnType)
		}

		at := models.AttachmentType{
			AttachmentMetaId: fm.AttachmentMetaId,
			ProfileId:        authorID,
			ItemTypeId:       assocItemTypeID,
			ItemId:           assoc.OnID,
			FileHash:         SHA1,
			FileName:         srcAttach.Name,
			Created:          srcAttach.DateCreated,
		}
		_, err = at.Import()
		if err != nil {
			glog.Errorf("Could not import attachment %d\n", srcAttach.ID)
			return err
		}

		tx, err := h.GetTransaction()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		fmt.Println(args.ItemTypeID)
		err = accounting.RecordImport(
			tx,
			args.OriginID,
			args.ItemTypeID,
			srcAttach.ID,
			at.AttachmentMetaId,
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

	return nil
}
