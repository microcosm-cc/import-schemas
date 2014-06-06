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
	// Get the site if it exists
	siteCreatedByUs := false
	siteID, adminID = GetExistingSiteAndAdmin(config.SiteSubdomainKey)
	if siteID == 0 {
		// Doesn't exist, so create the admin user
		tx, err := h.GetTransaction()
		if err != nil {
			log.Fatal(err)
		}
		defer tx.Rollback()

		userID, err := StoreUser(tx, owner)
		if err != nil {
			log.Fatal(err)
		}

		err = tx.Commit()
		if err != nil {
			log.Fatal(err)
		}

		// Use create_owned_site which will create the site and owner's profile.
		siteID, adminID, err = CreateOwnedSite(owner.Name, userID, Site{
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

	tx2, err := h.GetTransaction()
	if err != nil {
		log.Fatal(err)
	}
	defer tx2.Rollback()

	originID = GetImportInProgress(siteID, config.SiteName)
	if originID == 0 {
		log.Println("Commencing import")
		// Create an import origin.
		originID, err = accounting.CreateImportOrigin(
			tx2,
			config.SiteName,
			siteID,
		)
		if err != nil {
			log.Fatal(err)
		}

	} else {
		log.Println("Resuming import")
		accounting.LoadPriorImports(originID)
	}

	if siteCreatedByUs {
		// Record the import of the site owner.
		err = accounting.RecordImport(
			tx2,
			originID,
			h.ItemTypes[h.ItemTypeProfile],
			owner.ID,
			adminID,
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = tx2.Commit()
	if err != nil {
		log.Fatal(err)
	}

	// Finalise the site creation and import initialisation

	return originID, siteID, adminID
}

func GetExistingSiteAndAdmin(
	subdomainKey string,
) (
	siteID int64,
	adminID int64,
) {
	db, err := h.GetConnection()
	if err != nil {
		log.Fatal(err)
	}

	err = db.QueryRow(`
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

	switch {
	case err == sql.ErrNoRows:
		return
	case err != nil:
		log.Fatal(err)
	}

	return
}

func GetImportInProgress(
	siteID int64,
	originTitle string,
) (
	originID int64,
) {
	db, err := h.GetConnection()
	if err != nil {
		log.Fatal(err)
	}

	err = db.QueryRow(`
SELECT origin_id
  FROM import_origins
 WHERE site_id = $1`,
		siteID,
	).Scan(
		&originID,
	)

	switch {
	case err == sql.ErrNoRows:
		return
	case err != nil:
		log.Fatal(err)
	}

	return
}

func CreateOwnedSite(
	ownerName string,
	userId int64,
	site Site,
) (
	siteId int64,
	profileId int64,
	err error,
) {
	db, err := h.GetConnection()
	if err != nil {
		log.Fatal(err)
	}

	// Create simple profile for site owner.
	profile := Profile{
		ProfileName: ownerName,
		UserId:      userId,
	}

	err = db.QueryRow(`
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
