package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sort"

	_ "github.com/lib/pq"

	exports "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/config"
	"github.com/microcosm-cc/import-schemas/walk"
)

func exitWithError(fatal error, errors []error) {
	if len(errors) > 0 {
		log.Print("Encountered errors while importing:")
		for _, err := range errors {
			log.Print(err)
		}
	}
	log.Print("Fatal error:")
	log.Fatal(fatal)
}

func main() {

	h.InitDBConnection(h.DBConfig{
		Host:     config.DbHost,
		Port:     config.DbPort,
		Database: config.DbName,
		Username: config.DbUser,
		Password: config.DbPass,
	})

	// Collect non-fatal errors to print at the end.
	var errors []error

	// Load all users and create a single user entry corresponding to the site
	// owner.
	eOwner, eUsers, err := LoadUsers(config.Rootpath, config.SiteOwnerID)
	if err != nil {
		log.Fatal(err)
	}

	// Create the site and the admin user to initialise the import
	originID, iSiteID, iProfileID := CreateSiteAndAdminUser(eOwner)

	// Import all other users.
	pMap, pErrors := StoreUsers(originID, iSiteID, eUsers)
	errors = append(errors, pErrors...)

	// Import forums.
	fMap, fErrors := ImportForums(config.Rootpath, iSiteID, iProfileID, originID)
	errors = append(errors, fErrors...)

	// Conversations
	eConvMap, err := walk.WalkExports(config.Rootpath, "conversations")
	if err != nil {
		exitWithError(err, errors)
	}

	var cKeys []int
	for key := range eConvMap {
		cKeys = append(cKeys, key)
	}
	sort.Ints(cKeys)

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

	// Load comments.
	eCommMap, err := walk.WalkExports(config.Rootpath, "comments")
	if err != nil {
		exitWithError(err, errors)
	}

	var commentKeys []int
	for key := range eCommMap {
		commentKeys = append(commentKeys, key)
	}
	sort.Ints(commentKeys)

	for _, CommID := range commentKeys {
		bytes, err := ioutil.ReadFile(eCommMap[CommID])
		if err != nil {
			errors = append(errors, err)
			continue
		}
		eComm := exports.Comment{}
		err = json.Unmarshal(bytes, &eComm)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// Look up the author profile based on the old user ID.
		_, ok := pMap[eComm.Author]
		if !ok {
			errors = append(errors, fmt.Errorf(
				"Exported user ID %d does not have an imported profile, skipped comment %d\n",
				eComm.Author,
				CommID,
			))
			continue
		}

		// Look up the imported conversation ID based on the old ID. Assumes comments are only on conversations.
		_, ok = eConvMap[int(eComm.Association.OnID)]
		if !ok {
			errors = append(errors, fmt.Errorf(
				"Exported thread ID %d does not have an imported conversation, skipped comment %d\n",
				eComm.Association.OnID,
				CommID,
			))
			continue
		}

		// InReplyTo
		// Store comment.
	}

	log.Print(errors)
}
