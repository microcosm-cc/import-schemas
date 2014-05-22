package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/microcosm-cc/import-schemas/config"
	"log"
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
	owner, _, err := LoadUsers(config.Rootpath, config.SiteOwnerId)
	if err != nil {
		log.Fatal(err)
	}
	ownerId, err := StoreUser(db, owner)
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
	siteId, profileId, err := CreateOwnedSite(db, owner.Name, ownerId, site)
	log.Printf("New site ID: %d\n", siteId)
	log.Printf("Owner profile ID: %d\n", profileId)

	// Now, create an import origin.
	originId, err := CreateImportOrigin(db, site.Title, siteId)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Origin ID: %d\n", originId)

	// Now record the import of the site owner
	err = RecordImport(db, originId, ItemTypeUser, owner.Id, ownerId)
	if err != nil {
		log.Fatal(err)
	}

	// Finally, store the rest of the users, creating profiles for each of them.

}
