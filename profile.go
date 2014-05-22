package main

import (
	"database/sql"
	exports "github.com/microcosm-cc/export-schemas/go/forum"
)

func StoreProfile(db *sql.DB, siteId int64, user exports.User) error {

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
            1, NOW(), NOW(), 1, ''
        );`,
		siteId,
		user.Id,
		user.Name,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}
