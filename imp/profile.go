package imp

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/cheggaaa/pb"

	exports "github.com/microcosm-cc/export-schemas/go/forum"
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

// LoadUsers from JSON files into exports.User structs and returns the owner
// (as specified in the config file) separately.
func LoadUsers(rootPath string, ownerID int64) (exports.User, error) {

	itemTypeID := h.ItemTypes[h.ItemTypeProfile]

	err := files.WalkExportTree(rootPath, itemTypeID)
	if err != nil {
		return exports.User{}, err
	}

	// Does the owner, as specified in the config file, actually exist in the
	// files that we've discovered? If no, throw an error, and if yes return it.
	ownerPath := files.GetPath(itemTypeID, ownerID)
	if ownerPath == "" {
		return exports.User{},
			fmt.Errorf("Owner (from config) not found (within exported users)")
	}

	owner := exports.User{}
	err = files.JSONFileToInterface(ownerPath, &owner)
	if err != nil {
		return exports.User{}, err
	}

	return owner, nil
}

// createUser stores a single user, but does not create an associated profile.
// If an existing user is found in Microcosm with the same email address, we
// return that
func createUser(tx *sql.Tx, user exports.User) (int64, error) {

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

// ImportProfiles iterates a range of exports.Users and imports each
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

	// Read user from disk
	//
	// Done here so that if we are resuming and only a few failed we only end up
	// reading a few things from disk rather than everything.
	user := exports.User{}
	err := files.JSONFileToInterface(files.GetPath(itemTypeID, itemID), &user)
	if err != nil {
		return err
	}

	tx, err := h.GetTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	userID, err := createUser(tx, user)
	if err != nil {
		return err
	}

	// Create a corresponding profile for the user.
	avatarURL := sql.NullString{
		String: "/api/v1/files/66cca61feb8001cb71a9fb7062ff94c9d2543340",
		Valid:  true,
	}
	profile := Profile{
		SiteID:            siteID,
		UserID:            userID,
		ProfileName:       user.Name,
		AvatarURLNullable: avatarURL,
	}
	iProfileID, err := createProfile(tx, profile)
	if err != nil {
		return err
	}

	err = accounting.RecordImport(
		tx,
		originID,
		h.ItemTypes[h.ItemTypeProfile],
		user.ID,
		iProfileID,
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
