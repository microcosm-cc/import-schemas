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

func CreateSiteAndAdminUser(eOwner exports.User) (int64, int64, int64) {
	tx, err := h.GetTransaction()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	iOwnerID, err := StoreUser(tx, eOwner)
	if err != nil {
		log.Fatal(err)
	}

	// Use create_owned_site which will create the site and owner's profile.
	site := Site{
		Title:        config.SiteName,
		SubdomainKey: config.SiteSubdomainKey,
		Description:  config.SiteDesc,
		ThemeId:      1,
	}
	iSiteID, iProfileID, err := CreateOwnedSite(tx, eOwner.Name, iOwnerID, site)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created new site: %s, ID: %d\n", site.Title, iSiteID)
	log.Printf("Owner profile ID: %d\n", iProfileID)

	// Create an import origin.
	originID, err := accounting.CreateImportOrigin(tx, site.Title, iSiteID)
	if err != nil {
		log.Fatal(err)
	}

	// Record the import of the site owner.
	err = accounting.RecordImport(
		tx,
		originID,
		h.ItemTypes[h.ItemTypeUser],
		eOwner.ID,
		iOwnerID,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Finalise the site creation and import initialisation
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	return originID, iSiteID, iProfileID
}

func CreateOwnedSite(tx *sql.Tx, ownerName string, userId int64, site Site) (siteId int64, profileId int64, err error) {

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
