package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	exports "github.com/microcosm-cc/export-schemas/go/forum"
	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/config"
	"github.com/microcosm-cc/import-schemas/walk"
	"io/ioutil"
	"log"
	"sort"
)

const (
	ItemTypeMicrocosm    int64 = 2
	ItemTypeProfile      int64 = 3
	ItemTypeComment      int64 = 4
	ItemTypeConversation int64 = 6
	ItemTypeUser         int64 = 14
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

	// Collect non-fatal errors to print at the end.
	var errors []error

	connString := fmt.Sprintf("user=%s dbname=%s host=%s port=%d password=%s sslmode=disable",
		config.DbUser, config.DbName, config.DbHost, config.DbPort, config.DbPass)
	db, err := sql.Open("postgres", connString)
	if err != nil {
		log.Fatal(err)
	}

	// Check we have a good connection before doing anything else.
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// Load all users and create a single user entry corresponding to the site owner.
	eOwner, eUsers, err := LoadUsers(config.Rootpath, config.SiteOwnerId)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Loaded %d users\n", len(eUsers))
	iOwnerId, err := StoreUser(db, eOwner)
	if err != nil {
		log.Fatal(err)
	}

	// Use create_owned_site which will create the site and owner's profile.
	site := Site{
		Title:        config.SiteName,
		SubdomainKey: config.SiteSubdomainKey,
		Description:  config.SiteDesc,
		ThemeId:      1,
	}
	iSiteId, iProfileId, err := CreateOwnedSite(db, eOwner.Name, iOwnerId, site)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created new site: %s, ID: %d\n", site.Title, iSiteId)
	log.Printf("Owner profile ID: %d\n", iProfileId)

	// Create an import origin.
	originId, err := accounting.CreateImportOrigin(db, site.Title, iSiteId)
	if err != nil {
		log.Fatal(err)
	}

	// Record the import of the site owner.
	err = accounting.RecordImport(db, originId, ItemTypeUser, eOwner.ID, iOwnerId)
	if err != nil {
		log.Fatal(err)
	}

	// Import all other users.
	pMap, pErrors := StoreUsers(db, iSiteId, originId, eUsers)
	errors = append(errors, pErrors...)

	// Import forums.
	fMap, fErrors := ImportForums(db, config.Rootpath, iSiteId, iProfileId, originId)
	errors = append(errors, fErrors...)

	// Conversations
	eConvMap, err := walk.WalkExports(config.Rootpath, "conversations")
	if err != nil {
		exitWithError(err, errors)
	}

	var cKeys []int
	for key, _ := range eConvMap {
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
		authorId, ok := pMap[eConv.Author]
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
			CreatedBy:   authorId,
			ViewCount:   eConv.ViewCount,
			IsSticky:    false,
			IsOpen:      true,
			IsDeleted:   false,
			IsModerated: false,
			IsVisible:   true,
		}
		iCID, err := StoreConversation(db, c)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		err = accounting.RecordImport(db, originId, ItemTypeConversation, eConv.ID, iCID)
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
	for key, _ := range eCommMap {
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
