package imp

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/cheggaaa/pb"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
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

// LoadUsers from JSON files into src.Profile structs and returns the owner
// (as specified in the config file) separately.
func LoadUsers(rootPath string, ownerID int64) (src.Profile, error) {

	itemTypeID := h.ItemTypes[h.ItemTypeProfile]

	err := files.WalkExportTree(rootPath, itemTypeID)
	if err != nil {
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
		return src.Profile{}, err
	}

	return owner, nil
}

// createUser stores a single user, but does not create an associated profile.
// If an existing user is found in Microcosm with the same email address, we
// return that
func createUser(tx *sql.Tx, user src.Profile) (int64, error) {

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
		return 0, err
	}
	if userID > 0 {
		return userID, nil
	}

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

	return userID, err
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

// ImportProfiles iterates a range of src.Users and imports each
// individually
func ImportProfiles(
	siteID int64,
	originID int64,
) (
	errors []error,
) {

	log.Print("Importing profiles...")

	// Import users and create a profile for each.
	ids := files.GetIDs(h.ItemTypes[h.ItemTypeProfile])

	bar := pb.StartNew(len(ids))
	for _, id := range ids {
		err := importProfile(siteID, originID, id)
		if err != nil {
			errors = append(errors, err)
		}
		bar.Increment()
	}

	bar.Finish()

	return errors
}

func importProfile(siteID int64, originID int64, itemID int64) error {
	var itemTypeID = h.ItemTypes[h.ItemTypeProfile]

	// Skip when it already exists
	if accounting.GetNewID(originID, itemTypeID, itemID) > 0 {
		return nil
	}

	// Read profile from disk
	//
	// Done here so that if we are resuming and only a few failed we only end up
	// reading a few things from disk rather than everything.
	srcProfile := src.Profile{}
	err := files.JSONFileToInterface(files.GetPath(itemTypeID, itemID), &srcProfile)
	if err != nil {
		return err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	userID, err := createUser(tx, srcProfile)
	if err != nil {
		return err
	}

	// Create a corresponding profile for the srcProfile.
	avatarURL := sql.NullString{
		String: "/api/v1/files/66cca61feb8001cb71a9fb7062ff94c9d2543340",
		Valid:  true,
	}
	profile := Profile{
		SiteID:            siteID,
		UserID:            userID,
		ProfileName:       srcProfile.Name,
		AvatarURLNullable: avatarURL,
	}
	profileID, err := createProfile(tx, profile)
	if err != nil {
		return err
	}

	err = accounting.RecordImport(
		tx,
		originID,
		h.ItemTypes[h.ItemTypeProfile],
		srcProfile.ID,
		profileID,
	)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
