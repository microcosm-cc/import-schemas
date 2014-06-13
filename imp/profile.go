package imp

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

// profileLock is used to ensure that several different queries against
// microcosm models are forced into a single region of code. That is, we are
// forcing multiple functions that have their own transactions to run as a
// single block so that we do not encounter race conditions
var profileLock sync.Mutex

// loadProfiles from JSON files into the files/maps.go knowledge of what exists
// and returns the owner (as specified in the config file) as a src.Profile{}.
func loadProfiles(rootPath string, ownerID int64) (src.Profile, error) {

	itemTypeID := h.ItemTypes[h.ItemTypeProfile]

	err := files.WalkExportTree(rootPath, itemTypeID)
	if err != nil {
		glog.Errorf("Failed to walk tree: %+v", err)
		return src.Profile{}, err
	}

	// Does the owner, as specified in the config file, actually exist in the
	// files that we've discovered? If no, throw an error, and if yes return it.
	ownerPath := files.GetPath(itemTypeID, ownerID)
	if ownerPath == "" {
		return src.Profile{},
			fmt.Errorf("Owner (from config) not found (within exported users)")
	}

	owner := src.Profile{}
	err = files.JSONFileToInterface(ownerPath, &owner)
	if err != nil {
		glog.Errorf("Failed to load profile from JSON: %+v", err)
		return src.Profile{}, err
	}

	if glog.V(2) {
		glog.Infof("Found owner profile %d", owner.ID)
	}
	return owner, nil
}

// createDeletedProfile to own all of the orphaned content we find
func createDeletedProfile(args conc.Args) (int64, error) {

	sp := src.Profile{
		Email: "deleted@microcosm.cc",
		Name:  "deleted",
	}

	profileID, err := createProfile(args, sp)
	if err != nil {
		glog.Errorf("Failed to get existing user by email address: %+v", err)
		return 0, err
	}

	accounting.AddDeletedProfileID(profileID)

	if glog.V(2) {
		glog.Infof("Successfully create deleted profile %d", profileID)
	}
	return profileID, nil
}

// importProfiles iterates the profiles and imports each individually
func importProfiles(args conc.Args, gophers int) (errors []error) {

	fmt.Println("Importing profiles...")
	glog.Info("Importing profiles...")

	args.ItemTypeID = h.ItemTypes[h.ItemTypeProfile]

	// Import users and create a profile for each.
	return conc.RunTasks(
		files.GetIDs(args.ItemTypeID),
		args,
		importProfile,
		gophers,
	)
}

func importProfile(args conc.Args, itemID int64) error {
	// Skip when it already exists
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping profile %d", itemID)
		}
		return nil
	}

	// Read profile from disk
	//
	// Done here so that if we are resuming and only a few failed we only end up
	// reading a few things from disk rather than everything.
	sp := src.Profile{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&sp,
	)
	if err != nil {
		glog.Errorf("Failed to load profile from JSON: %+v", err)
		return err
	}

	profileID, err := createProfile(args, sp)
	if err != nil {
		glog.Errorf("Failed to createProfile %d : %+v", itemID, err)
		return err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to get transaction: %+v", err)
		return err
	}
	defer tx.Rollback()

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		sp.ID,
		profileID,
	)
	if err != nil {
		glog.Errorf("Failed to recordImport: %+v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		glog.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	if glog.V(2) {
		glog.Infof("Successfully imported profile %d", itemID)
	}
	return nil
}

// createProfile puts a profile into the database via microcosm models
func createProfile(args conc.Args, sp src.Profile) (int64, error) {

	profileLock.Lock()
	defer profileLock.Unlock()

	u, status, err := models.GetUserByEmailAddress(sp.Email)
	if err != nil && status != http.StatusNotFound {
		glog.Errorf(
			"Failed to get existing user by email address: <%s> %+v",
			sp.Email,
			err,
		)
		return 0, err
	}
	if status == http.StatusNotFound {
		// User doesn't exist, so we should create it
		u = models.UserType{}
		u.Email = sp.Email
		u.Language = "en-GB"

		_, err := u.Insert()
		if err != nil {
			glog.Errorf("Failed to create user for profile: %+v", err)
			return 0, err
		}
	} else {
		// User does exist, so we should check whether that user already has a
		// profile on this site and return that.
		profileID, status, err := models.GetProfileId(args.SiteID, u.ID)
		if err != nil && status != http.StatusNotFound {
			glog.Errorf("Failed to get existing profile for user: %+v", err)
			return 0, err
		}
		if status == http.StatusOK {
			// We already have a profile, return that
			return profileID, nil
		}
	}

	// We don't have a profile, but we do now have a user, so create the profile
	p := models.ProfileType{}
	p.SiteId = args.SiteID
	p.UserId = u.ID
	p.ProfileName = sp.Name
	p.Created = sp.DateCreated
	p.LastActive = sp.LastActive
	p.Visible = true
	p.StyleId = 1
	p.AvatarUrlNullable = sql.NullString{
		String: "/api/v1/files/66cca61feb8001cb71a9fb7062ff94c9d2543340",
		Valid:  true,
	}

	_, err = p.Import()
	if err != nil {
		glog.Errorf("Failed to create profile for profile %d: %+v", sp.ID, err)
		return 0, err
	}

	if glog.V(2) {
		glog.Infof("Successfully created profile %d", p.Id)
	}
	return p.Id, nil
}
