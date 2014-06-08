package imp

import (
	"database/sql"
	"fmt"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

// Profile struct
type Profile struct {
	ProfileName       string
	SiteID            int64
	UserID            int64
	ProfileID         int64
	AvatarIDNullable  sql.NullInt64
	AvatarURLNullable sql.NullString
}

// loadProfiles from JSON files into src.Profile structs and returns the owner
// (as specified in the config file) separately.
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

// importProfiles iterates a range of src.Users and imports each
// individually
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
	srcProfile := src.Profile{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&srcProfile,
	)
	if err != nil {
		glog.Errorf("Failed to load profile from JSON: %+v", err)
		return err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to get transaction: %+v", err)
		return err
	}
	defer tx.Rollback()

	userID, profileID, err := createUser(tx, args.SiteID, srcProfile)
	if err != nil {
		glog.Errorf("Failed to createUser for profile %d: %+v", itemID, err)
		return err
	}

	if profileID > 0 {
		// createUser reports that the user and a profile already existed
		err = accounting.RecordImport(
			tx,
			args.OriginID,
			args.ItemTypeID,
			srcProfile.ID,
			profileID,
		)
		if err != nil {
			glog.Errorf("Failed to recordImport: %+v", err)
			return err
		}

		if glog.V(2) {
			glog.Infof("Found profile %d for src profile %d", profileID, itemID)
		}
		return nil
	}

	// Create a corresponding profile for the srcProfile.
	avatarURL := sql.NullString{
		String: "/api/v1/files/66cca61feb8001cb71a9fb7062ff94c9d2543340",
		Valid:  true,
	}
	profile := Profile{
		SiteID:            args.SiteID,
		UserID:            userID,
		ProfileName:       srcProfile.Name,
		AvatarURLNullable: avatarURL,
	}
	profileID, err = createProfile(tx, profile)
	if err != nil {
		glog.Errorf("Failed to createProfile for profile %d: %+v", itemID, err)
		return err
	}

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		srcProfile.ID,
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

// createUser stores a single user, but does not create an associated profile.
// If an existing user is found in Microcosm with the same email address, we
// return that
func createUser(tx *sql.Tx, siteID int64, user src.Profile) (int64, int64, error) {
	// We may already have a user record based on this email
	var userID int64
	err := tx.QueryRow(`
SELECT user_id
  FROM users
 WHERE LOWER(email) = LOWER($1)`,
		user.Email,
	).Scan(
		&userID,
	)
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, err
	}
	if userID > 0 {
		// We have a user record already, but we might also have a profile on
		// this site for this user
		if siteID > 0 {
			var profileID int64
			err := tx.QueryRow(`
SELECT profile_id
  FROM profiles
 WHERE site_id = $1
   AND user_id = $2`,
				siteID,
				userID,
			).Scan(
				&profileID,
			)
			if err != nil && err != sql.ErrNoRows {
				return 0, 0, err
			}
			if profileID > 0 {
				// We already have a user and profile, return those
				return userID, profileID, nil
			}
		}

		// We have a user for another site, but no profiles on this one
		return userID, 0, nil
	}

	// We do not have a user or profile, create the user
	err = tx.QueryRow(`
INSERT INTO users (
    email, language, created, is_banned, password,
    password_date
) VALUES (
	$1, $2, $3, $4, '',
	NOW()
) RETURNING user_id;`,
		user.Email,
		"en-gb",
		user.DateCreated,
		user.Banned,
	).Scan(
		&userID,
	)

	return userID, 0, err
}

// createProfile puts a profile into the database
func createProfile(tx *sql.Tx, profile Profile) (profileID int64, err error) {

	err = tx.QueryRow(`
INSERT INTO profiles (
    site_id, user_id, profile_name, is_visible, style_id,
    created, last_active, avatar_id, avatar_url
) VALUES (
    $1, $2, $3, true, 1,
    NOW(), NOW(), NULL, $4
) RETURNING profile_id;`,
		profile.SiteID,
		profile.UserID,
		profile.ProfileName,
		profile.AvatarURLNullable,
	).Scan(
		&profileID,
	)

	return
}
