package imp

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/config"
)

// We should only use 50 gophers as we expect each one to trigger database
// connections and eventually we need this to run on a live site without
// breaking it
const gophers int = 50

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

	if glog.V(2) {
		glog.Infof("Args for import: %+v", args)
	}

	// Import all other users.
	// NOTE: Can only use 1 gopher as users may have multiple profiles and we
	// wish to only keep the oldest (lowest numbered) profile.
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

	// Import comments.
	errs = importComments(args, 25)
	if len(errs) > 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		glog.Flush()

		// If we have errors we do not continue. Errors importing comments
		// cascade significantly as attachments are associated to the comments.
		return
	}

	// Import follows.
	errs = importFollows(args, gophers)
	if len(errs) > 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		glog.Flush()
	}

	// Import messages as huddles.
	errs = importHuddles(args, 25)
	if len(errs) > 0 {
		for _, err := range errs {
			glog.Error(err)
		}
		glog.Flush()

		return
	}

	// TODO: Import messages here

	// TODO: Import attachments here
	//
	// Can be highly concurrent as nearly all activity here is going to be disk
	// and network limited... perhaps 100+ gophers?

	// TODO: Import roles here
	//
	// Roles must be the very last thing we do, as once we add permissions we
	// will be securing the site and a lot of the prior imports would fail if
	// the permissions today didn't precisely match the permissions at the time
	// that content was created... which is extremely unlikely.
	//
	// As a result, we shouldn't import roles until we are sure we got here
	// without error, as roles will reduce the overall resumability of an import

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
