package imp

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	"github.com/microcosm-cc/microcosm/audit"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

func importHuddles(args conc.Args, gophers int) (errors []error) {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeHuddle]

	fmt.Println("Importing messages")
	glog.Info("Importing messages")

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

func importHuddle(args conc.Args, itemID int64) error {

	// Skip message if already imported.
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping comment %d\n", itemID)
		}
		return nil
	}

	srcMessage := src.Message{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&srcMessage,
	)
	if err != nil {
		glog.Errorf("Failed to load message from JSON: %+v", err)
		return err
	}

	// Look up the author profile based on the old user ID.
	authorID := accounting.GetNewID(
		args.OriginID,
		h.ItemTypes[h.ItemTypeProfile],
		srcMessage.Author,
	)
	if authorID == 0 {
		authorID = args.DeletedProfileID
		if glog.V(2) {
			glog.Infof(
				"Using deleted profile for profile ID %d",
				srcMessage.Author,
			)
		}
	}

	huddle := models.HuddleType{
		Title:          srcMessage.Versions[0].Headline,
		IsConfidential: true,
	}
	huddle.Meta.Flags.Deleted = srcMessage.Deleted
	huddle.Meta.Created = srcMessage.DateCreated
	huddle.Meta.CreatedById = authorID

	// Internal accounting to make sure we don't add someone twice as vBulletin
	// allowed rubbish like that. The author is added internally to the
	// participants within microcosms, so start by adding the author.
	participants := make(map[int64]bool)
	participants[authorID] = true

	for _, to := range srcMessage.To {
		RID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeProfile],
			to.ID,
		)
		if RID > 0 {
			r := models.ProfileSummaryType{
				Id: RID,
			}

			if _, ok := participants[RID]; !ok {
				huddle.Participants = append(huddle.Participants, r)
				participants[RID] = true
			}
		}
	}
	for _, to := range srcMessage.BCC {
		RID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeProfile],
			to.ID,
		)
		if RID > 0 {
			r := models.ProfileSummaryType{
				Id: RID,
			}

			if _, ok := participants[RID]; !ok {
				huddle.Participants = append(huddle.Participants, r)
				participants[RID] = true
			}
		}
	}

	_, err = huddle.Import(args.SiteID)
	if err != nil {
		glog.Errorf("Failed to create huddle %d: %+v", itemID, err)
		return err
	}

	m := models.CommentSummaryType{
		ItemType: "huddle",
		ItemId:   huddle.Id,
		Markdown: srcMessage.Versions[0].Text,
	}
	m.Meta.Created = srcMessage.DateCreated
	m.Meta.CreatedById = authorID
	m.Meta.Flags.Deleted = srcMessage.Deleted
	m.Meta.Flags.Visible = !srcMessage.Deleted

	_, err = m.Import(args.SiteID)

	if err != nil {
		// Ignore errors relating to link embedding.
		if !strings.Contains(err.Error(), "links_url_key") {
			glog.Errorf("Failed to import comment for huddle %d: %s", srcMessage.ID, err)
			return err
		}
	}

	// Mark the sent version of the huddle as read
	_, err = models.MarkAsRead(h.ItemTypes[h.ItemTypeHuddle], huddle.Id, authorID, time.Now())

	// Mark the huddle as read.
	for _, p := range huddle.Participants {
		_, err := models.MarkAsRead(h.ItemTypes[h.ItemTypeHuddle], huddle.Id, p.Id, time.Now())
		if err != nil {
			glog.Infof("Could not MarkAsRead huddle %d for recipient %d", huddle.Id, p.Id)
		}
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
		srcMessage.ID,
		huddle.Id,
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
		h.ItemTypes[h.ItemTypeHuddle],
		huddle.Id,
		authorID,
		srcMessage.DateCreated,
		net.ParseIP(srcMessage.IPAddress),
	)

	return nil
}
