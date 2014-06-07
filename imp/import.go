package imp

import (
	"log"

	"github.com/microcosm-cc/import-schemas/config"
)

// Import orchestrates and runs the import job, ensuring that any dependencies
// are imported before they are needed. This is mostly a top level ordering of
// things: users before the comments they made, etc.
func Import() {
	// Collect non-fatal errors to print at the end.
	var errors []error

	// Load all users and create a single user entry corresponding to the site
	// owner.
	eOwner, err := LoadUsers(config.Rootpath, config.SiteOwnerID)
	if err != nil {
		log.Fatal(err)
	}

	// Create the site and the admin user to initialise the import
	originID, siteID, adminID := CreateSiteAndAdminUser(eOwner)

	// Import all other users.
	pErrors := ImportProfiles(siteID, originID)
	errors = append(errors, pErrors...)

	// Import forums.
	fErrors := ImportForums(config.Rootpath, siteID, adminID, originID)
	errors = append(errors, fErrors...)

	// Import conversations.
	cErrors := ImportConversations(config.Rootpath, siteID, originID)
	errors = append(errors, cErrors...)

	log.Print(errors)
}

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
