package imp

import (
	"database/sql"
	"fmt"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/config"
)

// Site struct
type Site struct {
	Title              string
	SubdomainKey       string
	ThemeID            int64
	DomainNullable     string
	Description        string
	LogoURL            string
	BackgroundURL      string
	BackgroundPosition string
	BackgroundColor    string
	LinkColor          string
}

// createSiteAndAdminUser will either create or fetch the admin user and site
func createSiteAndAdminUser(
	owner src.Profile,
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
			glog.Fatal(err)
		}
		defer tx.Rollback()

		userID, _, err := createSiteUser(tx, 0, owner)
		if err != nil {
			glog.Fatal(err)
		}

		err = tx.Commit()
		if err != nil {
			glog.Fatal(err)
		}

		// Use create_owned_site which will create the site and owner's profile.
		siteID, adminID, err = CreateOwnedSite(owner.Name, userID, Site{
			Title:        config.SiteName,
			SubdomainKey: config.SiteSubdomainKey,
			Description:  config.SiteDesc,
			ThemeID:      1,
		})
		if err != nil {
			glog.Fatal(err)
		}

		fmt.Printf(
			"Importing into NEW site\n"+
				"Title: %s\n"+
				"ID: %d\n"+
				"OwnedBy: %d\n",
			config.SiteName,
			siteID,
			adminID,
		)

		siteCreatedByUs = true
	} else {
		fmt.Printf(
			"Importing into EXISTING site\n"+
				"Title: %s\n"+
				"ID: %d\n"+
				"OwnedBy: %d\n",
			config.SiteName,
			siteID,
			adminID,
		)
	}

	tx2, err := h.GetTransaction()
	if err != nil {
		glog.Fatal(err)
	}
	defer tx2.Rollback()

	originID = GetImportInProgress(siteID, config.SiteName)
	if originID == 0 {
		fmt.Println("Commencing import")
		// Create an import origin.
		originID, err = accounting.CreateImportOrigin(
			tx2,
			config.SiteName,
			siteID,
		)
		if err != nil {
			glog.Fatal(err)
		}

	} else {
		fmt.Println("Resuming import")
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
			glog.Fatal(err)
		}
	}

	err = tx2.Commit()
	if err != nil {
		glog.Fatal(err)
	}

	// Finalise the site creation and import initialisation

	return originID, siteID, adminID
}

// GetExistingSiteAndAdmin fetches an existing site, matching on the subdomain
func GetExistingSiteAndAdmin(
	subdomainKey string,
) (
	siteID int64,
	adminID int64,
) {
	db, err := h.GetConnection()
	if err != nil {
		glog.Fatal(err)
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
		glog.Fatal(err)
	}

	return
}

// GetImportInProgress looks up the site in the import_origins table
func GetImportInProgress(
	siteID int64,
	originTitle string,
) (
	originID int64,
) {
	db, err := h.GetConnection()
	if err != nil {
		glog.Fatal(err)
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
		glog.Fatal(err)
	}

	return
}

// CreateOwnedSite creates a site and returns that and the admin profile
func CreateOwnedSite(
	ownerName string,
	userID int64,
	site Site,
) (
	siteID int64,
	profileID int64,
	err error,
) {
	db, err := h.GetConnection()
	if err != nil {
		glog.Fatal(err)
	}

	// Create site and simple profile for site owner.
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
		site.ThemeID,
		userID,
		ownerName,

		sql.NullInt64{},
		sql.NullString{},
		site.DomainNullable,
		site.Description,
		site.LogoURL,

		site.BackgroundURL,
		site.BackgroundPosition,
		site.BackgroundColor,
		site.LinkColor,
	).Scan(
		&siteID,
		&profileID,
	)
	if err != nil {
		glog.Fatal(err)
	}

	return
}

// createSiteUser stores a single user, but does not create an associated
// profile. If an existing user is found in Microcosm with the same email
// address, we return that
func createSiteUser(
	tx *sql.Tx,
	siteID int64,
	user src.Profile,
) (
	int64,
	int64,
	error,
) {
	// We may already have a user record based on this email
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
		return 0, 0, err
	}
	if userID > 0 {
		// We have a user record already, but we might also have a profile on
		// this site for this user
		if siteID > 0 {
			var profileID int64
			err := tx.QueryRow(`
SELECT profile_id
  FROM profiles
 WHERE site_id = $1
   AND user_id = $2`,
				siteID,
				userID,
			).Scan(
				&profileID,
			)
			if err != nil && err != sql.ErrNoRows {
				return 0, 0, err
			}
			if profileID > 0 {
				// We already have a user and profile, return those
				return userID, profileID, nil
			}
		}

		// We have a user for another site, but no profiles on this one
		return userID, 0, nil
	}

	// We do not have a user or profile, create the user
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

	return userID, 0, err
}
