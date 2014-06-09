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
	originID, siteID, adminProfileID := createSiteAndAdminUser(srcAdminProfile)

	// Create args for all concurrent jobs, these are basically shared values
	// to help normalise the signature of tasks so that we can run lots of
	// different tasks as if the functions had the same signature.
	args := conc.Args{
		RootPath:           config.Rootpath,
		OriginID:           originID,
		SiteID:             siteID,
		SiteOwnerProfileID: adminProfileID,
	}

	// Create a user for orphaned content
	deletedProfileID, err := createDeletedProfile(args)
	if err != nil {
		glog.Fatal(err)
	}
	args.DeletedProfileID = deletedProfileID

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

	errs = importComments(args)
	if len(errs) > 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		glog.Flush()
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
