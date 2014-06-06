package main

import (
	"database/sql"
)

type Profile struct {
	ProfileName       string
	SiteId            int64
	UserId            int64
	ProfileId         int64
	AvatarIdNullable  sql.NullInt64
	AvatarUrlNullable sql.NullString
}

func StoreProfile(tx *sql.Tx, profile Profile) (profileID int64, err error) {

	err = tx.QueryRow(`
INSERT INTO profiles (
    site_id, user_id, profile_name, is_visible,
    style_id, created, last_active, avatar_id, avatar_url
) VALUES (
    $1, $2, $3, true,
    1, NOW(), NOW(), NULL, $4
) RETURNING profile_id;`,
		profile.SiteId,
		profile.UserId,
		profile.ProfileName,
		profile.AvatarUrlNullable,
	).Scan(
		&profileID,
	)

	return
}
