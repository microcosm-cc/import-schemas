package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"time"

	exports "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/walk"
)

type Conversation struct {
	ConversationId int64
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

func ImportConversations(rootpath string, iSiteID int64, pMap map[int64]int64, fMap map[int]int64, originID int64) (cMap map[int]int64, errors []error) {

	eConvMap, err := walk.WalkExports(rootpath, "conversations")
	if err != nil {
		errors = append(errors, err)
		return
	}

	var cKeys []int
	for key := range eConvMap {
		cKeys = append(cKeys, key)
	}
	sort.Ints(cKeys)
	cMap = make(map[int]int64)

	// Iterate conversations in order.
	for _, CID := range cKeys {
		bytes, err := ioutil.ReadFile(eConvMap[CID])
		if err != nil {
			errors = append(errors, err)
			continue
		}
		eConv := exports.Conversation{}
		err = json.Unmarshal(bytes, &eConv)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// Look up the author profile based on the old user ID.
		authorI, ok := pMap[eConv.Author]
		if !ok {
			errors = append(errors, fmt.Errorf(
				"Exported user ID %d does not have an imported profile, skipped conversation %d\n",
				eConv.Author,
				CID,
			))
			continue
		}

		MID, ok := fMap[int(eConv.ForumID)]
		if !ok {
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
	return
}

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
