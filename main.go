package main

import (
	"log"

	_ "github.com/lib/pq"

	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/config"
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
	pMap, pErrors := StoreUsers(iSiteID, originID, eUsers)
	errors = append(errors, pErrors...)

	// Import forums.
	fMap, fErrors := ImportForums(config.Rootpath, iSiteID, iProfileID, originID)
	errors = append(errors, fErrors...)

	// Import conversations.
	_, cErrors := ImportConversations(config.Rootpath, iSiteID, pMap, fMap, originID)
	errors = append(errors, cErrors...)

	log.Print(errors)
}
