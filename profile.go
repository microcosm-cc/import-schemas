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

func StoreProfile(db *sql.DB, profile Profile) error {

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`INSERT INTO profiles (
            site_id, user_id, profile_name, is_visible,
            style_id, created, last_active, avatar_id, avatar_url
        ) VALUES (
            $1, $2, $3, true,
            1, NOW(), NOW(), NULL, $4
        );`,
		profile.SiteId,
		profile.UserId,
		profile.ProfileName,
		profile.AvatarUrlNullable,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}
