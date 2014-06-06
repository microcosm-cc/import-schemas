package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cheggaaa/pb"

	exports "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
)

const usersPath string = "users"

func storeID(itemType string, path string, itemMap map[int]string) {
	// Split path on itemType (e.g. "users")
	strID := strings.Split(path, itemType)[1]
	// Split file extension
	strID = strings.Split(strID, ".json")[0]
	// Replace path delimiters to extract ID.
	strID = strings.Replace(strID, "/", "", -1)

	// Store ID and associated file path in itemMap.
	ID, err := strconv.Atoi(strID)
	if err != nil {
		log.Print(err)
	}
	itemMap[ID] = path
}

// LoadUsers from JSON files into exports.User structs and returns the owner
// (as specified in the config file) separately.
func LoadUsers(
	rootpath string,
	ownerID int64,
) (
	owner exports.User,
	users []exports.User,
	err error,
) {

	// Build a map of User ID -> file path.
	userMap := make(map[int]string)

	// Define what should be done in the subsequent directory walk.
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return err
		}
		if filepath.Ext(path) == ".json" {
			storeID(usersPath, path, userMap)
		}
		return nil
	}

	// Walk each path in the "users" directory.
	filepath.Walk(path.Join(rootpath, usersPath), walkFunc)

	var keys []int
	for key := range userMap {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	// Iterate map in order
	for _, ID := range keys {
		bytes, err := ioutil.ReadFile(userMap[ID])
		if err != nil {
			log.Printf("Error opening path: %d\n", ID)
			continue
		}
		exUser := exports.User{}
		err = json.Unmarshal(bytes, &exUser)
		if err != nil {
			log.Print(err)
			continue
		}

		if exUser.ID == ownerID {
			owner = exUser
		} else {
			users = append(users, exUser)
		}
	}

	return
}

// StoreUser stores a single user, but does not create an associated profile.
// If an existing user is found in Microcosm with the same email address, we
// return that
func StoreUser(tx *sql.Tx, user exports.User) (int64, error) {

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
		return userID, err
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

// StoreUsers iterates a range of exports.Users and imports each individually
func StoreUsers(
	siteID int64,
	originID int64,
	eUsers []exports.User,
) (
	errors []error,
) {

	log.Print("Importing users...")

	// Import users and create a profile for each.
	bar := pb.StartNew(len(eUsers))
	for _, user := range eUsers {

		bar.Increment()

		// Skip when it already exists
		if accounting.GetNewID(
			originID,
			h.ItemTypes[h.ItemTypeProfile],
			user.ID,
		) > 0 {
			continue
		}

		tx, err := h.GetTransaction()
		if err != nil {
			errors = append(errors, err)
			return
		}
		defer tx.Rollback()

		userID, err := StoreUser(tx, user)
		if err != nil {
			errors = append(errors, err)
			continue
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
		iProfileID, err := StoreProfile(tx, profile)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		err = accounting.RecordImport(
			tx,
			originID,
			h.ItemTypes[h.ItemTypeProfile],
			user.ID,
			iProfileID,
		)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		err = tx.Commit()
		if err != nil {
			errors = append(errors, err)
			continue
		}
	}

	bar.Finish()

	return errors
}
