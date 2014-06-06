package main

import (
	"database/sql"
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

// StoreProfile puts a profile into the database
func StoreProfile(tx *sql.Tx, profile Profile) (profileID int64, err error) {

	err = tx.QueryRow(`
INSERT INTO profiles (
    site_id, user_id, profile_name, is_visible,
    style_id, created, last_active, avatar_id, avatar_url
) VALUES (
    $1, $2, $3, true,
    1, NOW(), NOW(), NULL, $4
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
