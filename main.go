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
)

func main() {
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

	// Load all users, then create a single user entry corresponding to the site owner.
	exOwner, users, err := LoadUsers(config.Rootpath, config.SiteOwnerId)
	if err != nil {
		log.Fatal(err)
	}
	ownerId, err := StoreUser(db, exOwner)
	if err != nil {
		log.Fatal(err)
	}

	// Then, use create_owned_site which will create the site and owner's profile.
	site := Site{
		Title:        config.SiteName,
		SubdomainKey: config.SiteSubdomainKey,
		Description:  config.SiteDesc,
		ThemeId:      1,
	}
	siteId, profileId, err := CreateOwnedSite(db, exOwner.Name, ownerId, site)
	log.Printf("New site ID: %d\n", siteId)
	log.Printf("Owner profile ID: %d\n", profileId)

	// Now, create an import origin.
	originId, err := CreateImportOrigin(db, site.Title, siteId)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Origin ID: %d\n", originId)

	// Now record the import of the site owner
	err = RecordImport(db, originId, ItemTypeUser, exOwner.ID, ownerId)
	if err != nil {
		log.Fatal(err)
	}

	// Store the remaining users and create a profile for each.
	importedProfiles := make(map[int64]int64)

	for _, user := range users {
		newUserId, err := StoreUser(db, user)
		if err != nil {
			log.Print(err)
		}
		err = RecordImport(db, originId, ItemTypeUser, user.ID, newUserId)
		if err != nil {
			log.Print(err)
		}
		avatarUrl := sql.NullString{
			String: "/api/v1/files/66cca61feb8001cb71a9fb7062ff94c9d2543340",
			Valid:  true,
		}
		profile := Profile{
			SiteId:            siteId,
			UserId:            newUserId,
			ProfileName:       user.Name,
			AvatarUrlNullable: avatarUrl,
		}
		profileID, err := StoreProfile(db, profile)
		if err != nil {
			log.Print(err)
		}

		importedProfiles[user.ID] = profileID
		fmt.Printf(".")
	}

	// Forums
	fMap, err := walk.WalkExports(config.Rootpath, "forums")
	var fKeys []int
	for key, _ := range fMap {
		fKeys = append(fKeys, key)
	}
	sort.Ints(fKeys)

	for _, ID := range fKeys {
		bytes, err := ioutil.ReadFile(fMap[ID])
		if err != nil {
			log.Printf("Error opening path: %d\n", ID)
			continue
		}
		exForum := exports.Forum{}
		err = json.Unmarshal(bytes, &exForum)
		if err != nil {
			log.Print(err)
			continue
		}

		// CreatedBy/OwnedBy are assumed to be the site owner.
		m := Microcosm{
			Title:       exForum.Name,
			Description: exForum.Text,
			CreatedBy:   ownerId,
			OwnedBy:     ownerId,
			IsOpen:      exForum.Open,
			IsSticky:    exForum.Sticky,
			IsModerated: exForum.Moderated,
			IsDeleted:   exForum.Deleted,
		}
		mID, err := StoreMicrocosm(db, m)
		if err != nil {
			log.Print(err)
			continue
		}
		log.Print(mID)
		err = RecordImport(db, originId, ItemTypeMicrocosm, exForum.ID, mID)
		log.Print(err)
	}

	// Conversations
	/*
		    cMap, err := walk.WalkExports(config.Rootpath, "conversations")

			var keys []int
			for key, _ := range cMap {
				keys = append(keys, key)
			}
			sort.Ints(keys)

			// Iterate map in order
			for _, ID := range keys {
				bytes, err := ioutil.ReadFile(exportsMap[ID])
				if err != nil {
					log.Printf("Error opening path: %d\n", ID)
					continue
				}
				exConv := exports.Conversation{}
				err = json.Unmarshal(bytes, &exConv)
				if err != nil {
					log.Print(err)
					continue
				}

			}
	*/

}
