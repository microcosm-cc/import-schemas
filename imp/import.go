package imp

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/config"
)

const gophers int = 100

// Import orchestrates and runs the import job, ensuring that any dependencies
// are imported before they are needed. This is mostly a top level ordering of
// things: profiles before the comments they made, etc.
func Import() {
	// Load all profiles and create a single user entry corresponding to the site
	// admin.
	srcAdminProfile, err := loadProfiles(config.Rootpath, config.SiteOwnerID)
	if err != nil {
		glog.Fatal(err)
	}

	// Create the site and the admin user to initialise the import
	originID, siteID := createSiteAndAdminUser(srcAdminProfile)

	args := conc.Args{
		RootPath: config.Rootpath,
		OriginID: originID,
		SiteID:   siteID,
	}

	// Import all other users.
	errs := importProfiles(args, gophers)
	if len(errs) > 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		glog.Flush()

		// If we have errors we do not continue. Errors importing profiles
		// cascade significantly as everything is associated to a profile.
		return
	}

	// Import microcosms.
	errs = importMicrocosms(args, gophers)
	if len(errs) > 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		glog.Flush()

		// If we have errors we do not continue. Errors importing forums
		// cascade significantly as conversations are associated to the forums.
		return
	}

	// Import conversations.
	errs = importConversations(args, gophers)
	if len(errs) > 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		glog.Flush()

		// If we have errors we do not continue. Errors importing conversations
		// cascade significantly as comments are associated to the conversations.
		return
	}

}

func exitWithError(fatal error, errors []error) {
	if len(errors) > 0 {
		fmt.Println("Encountered errors while importing. Please check logs")

		for _, err := range errors {
			glog.Error(err)
		}
	}

	glog.Fatal(fatal)
}
