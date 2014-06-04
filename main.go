package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	exports "github.com/microcosm-cc/export-schemas/go/forum"
	"github.com/microcosm-cc/import-schemas/config"
	"github.com/microcosm-cc/import-schemas/walk"
	"io/ioutil"
	"log"
	"sort"
	"time"
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
	originId, err := CreateImportOrigin(db, site.Title, iSiteId)
	if err != nil {
		log.Fatal(err)
	}

	// Record the import of the site owner.
	err = RecordImport(db, originId, ItemTypeUser, eOwner.ID, iOwnerId)
	if err != nil {
		log.Fatal(err)
	}

	log.Print("Importing users...")
	// Map imported User ID to new Profile ID.
	pMap := make(map[int64]int64)
	// Import the remaining users and create a profile for each.
	for idx, user := range eUsers {

		iUserId, err := StoreUser(db, user)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		err = RecordImport(db, originId, ItemTypeUser, user.ID, iUserId)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// Create a corresponding profile for the user.
		avatarUrl := sql.NullString{
			String: "/api/v1/files/66cca61feb8001cb71a9fb7062ff94c9d2543340",
			Valid:  true,
		}
		profile := Profile{
			SiteId:            iSiteId,
			UserId:            iUserId,
			ProfileName:       user.Name,
			AvatarUrlNullable: avatarUrl,
		}
		iProfileID, err := StoreProfile(db, profile)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		pMap[user.ID] = iProfileID

		if idx%10 == 0 {
			fmt.Printf(".")
		}
	}
	fmt.Print("\n")

	// Forums
	log.Print("Importing forums...")
	eForumMap, err := walk.WalkExports(config.Rootpath, "forums")
	if err != nil {
		exitWithError(err, errors)
	}
	var fKeys []int
	for key, _ := range eForumMap {
		fKeys = append(fKeys, key)
	}
	sort.Ints(fKeys)

	// Map of Forum ID to new Microcosm ID.
	fMap := make(map[int]int64)

	for _, FID := range fKeys {
		bytes, err := ioutil.ReadFile(eForumMap[FID])
		if err != nil {
			errors = append(errors, err)
			continue
		}
		eForum := exports.Forum{}
		err = json.Unmarshal(bytes, &eForum)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// CreatedBy and OwnedBy are assumed to be the site owner.
		m := Microcosm{
			SiteId:      iSiteId,
			Title:       eForum.Name,
			Description: eForum.Text,
			Created:     time.Now(),
			CreatedBy:   iProfileId,
			OwnedBy:     iProfileId,
			IsOpen:      eForum.Open,
			IsSticky:    eForum.Sticky,
			IsModerated: eForum.Moderated,
			IsDeleted:   eForum.Deleted,
			IsVisible:   true,
		}
		MID, err := StoreMicrocosm(db, m)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		err = RecordImport(db, originId, ItemTypeMicrocosm, eForum.ID, MID)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		fMap[FID] = MID
		log.Printf("Created microcosm: %d\n", MID)
	}

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

		// TODO: Type conversion is spurious. Use a different key type.
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
		err = RecordImport(db, originId, ItemTypeConversation, eConv.ID, iCID)
		if err != nil {
			errors = append(errors, err)
			continue
		}
	}

}
