package main

import (
	"database/sql"
	"log"

	exports "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/config"
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

func CreateSiteAndAdminUser(
	owner exports.User,
) (
	originID int64,
	siteID int64,
	adminID int64,
) {
	tx, err := h.GetTransaction()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	// Get the site if it exists
	siteCreatedByUs := false
	siteID, adminID = GetExistingSiteAndAdmin(tx, config.SiteSubdomainKey)
	if siteID == 0 {
		// Doesn't exist, so create the admin user
		userID, err := StoreUser(tx, owner)
		if err != nil {
			log.Fatal(err)
		}

		// Use create_owned_site which will create the site and owner's profile.
		siteID, adminID, err := CreateOwnedSite(tx, owner.Name, userID, Site{
			Title:        config.SiteName,
			SubdomainKey: config.SiteSubdomainKey,
			Description:  config.SiteDesc,
			ThemeId:      1,
		})
		if err != nil {
			log.Fatal(err)
		}

		log.Printf(
			"Importing into NEW site\n"+
				"Title: %s\n"+
				"ID: %d\n"+
				"OwnedBy: %d",
			config.SiteName,
			siteID,
			adminID,
		)
		siteCreatedByUs = true
	} else {
		log.Printf(
			"Importing into EXISTING site\n"+
				"Title: %s\n"+
				"ID: %d\n"+
				"OwnedBy: %d",
			config.SiteName,
			siteID,
			adminID,
		)
	}

	originID = GetImportInProgress(tx, siteID, config.SiteName)
	if originID == 0 {
		// Create an import origin.
		originID, err = accounting.CreateImportOrigin(tx, config.SiteName, siteID)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Commencing import")
	} else {
		log.Println("Resuming import")
	}

	if siteCreatedByUs {
		// Record the import of the site owner.
		err = accounting.RecordImport(
			tx,
			originID,
			h.ItemTypes[h.ItemTypeUser],
			owner.ID,
			adminID,
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Finalise the site creation and import initialisation
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	return originID, siteID, adminID
}

func GetExistingSiteAndAdmin(
	tx *sql.Tx,
	subdomainKey string,
) (
	siteID int64,
	adminID int64,
) {

	err := tx.QueryRow(`
SELECT site_id
      ,owned_by
  FROM sites
 WHERE LOWER(subdomain_key) = LOWER($1)
`,
		subdomainKey,
	).Scan(
		&siteID,
		&adminID,
	)
	if err != nil {
		log.Fatal(err)
	}

	return
}

func GetImportInProgress(
	tx *sql.Tx,
	siteID int64,
	originTitle string,
) (
	originID int64,
) {

	err := tx.QueryRow(`
SELECT origin_id
  FROM import_origins
 WHERE site_id = $1
   AND LOWER(title) = LOWER($2)
`,
		siteID,
		originTitle,
	).Scan(
		&originID,
	)
	if err != nil {
		log.Fatal(err)
	}

	return
}

func CreateOwnedSite(
	tx *sql.Tx,
	ownerName string,
	userId int64,
	site Site,
) (
	siteId int64,
	profileId int64,
	err error,
) {

	// Create simple profile for site owner.
	profile := Profile{
		ProfileName: ownerName,
		UserId:      userId,
	}

	err = tx.QueryRow(`
SELECT new_ids.new_site_id
      ,new_ids.new_profile_id
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
	).Scan(
		&siteId,
		&profileId,
	)

	if err != nil {
		log.Fatal(err)
	}

	return
}
