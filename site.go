package main

import (
	"database/sql"
	"log"
)

type Site struct {
	Title              string
	SubdomainKey       string
	ThemeId            int64
	DomainNullable     string
	Description        string
	LogoUrl            string
	BackgroundUrl      string
	BackgroundPosition string
	BackgroundColor    string
	LinkColor          string
}

func CreateOwnedSite(db *sql.DB, ownerName string, userId int64, site Site) (siteId int64, profileId int64, err error) {

	// Create simple profile for site owner.
	profile := Profile{
		ProfileName: ownerName,
		UserId:      userId,
	}

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		log.Fatal(err)
	}

	err = tx.QueryRow(
		`SELECT
            new_ids.new_site_id,
            new_ids.new_profile_id
         FROM create_owned_site(
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9, $10,
            $11, $12, $13, $14
         ) AS new_ids`,
		site.Title,
		site.SubdomainKey,
		site.ThemeId,
		userId,

		profile.ProfileName,
		profile.AvatarIdNullable,
		profile.AvatarUrlNullable,

		site.DomainNullable,
		site.Description,
		site.LogoUrl,
		site.BackgroundUrl,
		site.BackgroundPosition,
		site.BackgroundColor,
		site.LinkColor,
	).Scan(&siteId, &profileId)

	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	return
}
