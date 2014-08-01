package imp

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	"github.com/microcosm-cc/microcosm/audit"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/config"
	"github.com/microcosm-cc/import-schemas/files"
)

// loadProfiles from JSON files into the files/maps.go knowledge of what exists
// and returns the owner (as specified in the config file) as a src.Profile{}.
func loadProfiles(rootPath string, ownerID int64) (src.Profile, error) {

	itemTypeID := h.ItemTypes[h.ItemTypeProfile]

	err := files.WalkExportTree(rootPath, itemTypeID)
	if err != nil {
		glog.Errorf("Failed to walk tree: %+v", err)
		return src.Profile{}, err
	}

	// Does the owner, as specified in the config file, actually exist in the
	// files that we've discovered? If no, throw an error, and if yes return it.
	ownerPath := files.GetPath(itemTypeID, ownerID)
	if ownerPath == "" {
		return src.Profile{},
			fmt.Errorf("Owner (from config) not found (within exported users)")
	}

	owner := src.Profile{}
	err = files.JSONFileToInterface(ownerPath, &owner)
	if err != nil {
		glog.Errorf("Failed to load profile from JSON: %+v", err)
		return src.Profile{}, err
	}

	if glog.V(2) {
		glog.Infof("Found owner profile %d", owner.ID)
	}
	return owner, nil
}

// createDeletedProfile to own all of the orphaned content we find
func createDeletedProfile(args conc.Args) (int64, error) {

	sp := src.Profile{
		Email: "deleted@microcosm.cc",
		Name:  "deleted",
	}

	profile, err := createProfile(args, sp)
	if err != nil {
		glog.Errorf("Failed to get existing user by email address: %+v", err)
		return 0, err
	}

	accounting.AddDeletedProfileID(profile.Id)

	if glog.V(2) {
		glog.Infof("Successfully create deleted profile %d", profile.Id)
	}
	return profile.Id, nil
}

// importProfiles iterates the profiles and imports each individually
func importProfiles(args conc.Args, gophers int) []error {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeProfile]

	// We may have an index.json file to guide us, if we do let's use it to
	// figure out our dupes and do things in two passes with high concurrency,
	// otherwise we need to do everything sequentially in a single process.

	indexFile := path.Join(config.Rootpath, src.ProfilesPath, "index.json")
	if !files.Exists(indexFile) {
		// Index file did not exist, sequential it is.
		fmt.Println("Importing profiles...")
		glog.Info("Importing profiles...")

		return conc.RunTasks(
			files.GetIDs(args.ItemTypeID),
			args,
			importProfile,
			1,
		)
	}

	fmt.Println("Importing profiles (2-pass)...")
	glog.Info("Importing profiles (2-pass)...")

	// indexFile exists, load it and figure out duplicates. Duplicates are
	// cases where the same email address has been used with multiple profiles.
	// These are merged into a single profile by the import process.
	di := src.DirIndex{}
	err := files.JSONFileToInterface(indexFile, &di)
	if err != nil {
		return []error{err}
	}

	emails := make(map[string]bool)
	firstPass := []int64{}
	secondPass := []int64{}
	for _, df := range di.Files {
		email := strings.ToLower(df.Email)
		if _, ok := emails[email]; ok {
			secondPass = append(secondPass, df.ID)
			continue
		}
		emails[email] = true
		firstPass = append(firstPass, df.ID)
	}

	errs := conc.RunTasks(firstPass, args, importProfile, gophers)
	return append(
		errs,
		conc.RunTasks(secondPass, args, importProfile, gophers)...,
	)
}

func importProfile(args conc.Args, itemID int64) error {
	// Skip when it already exists
	if accounting.GetNewID(args.OriginID, args.ItemTypeID, itemID) > 0 {
		if glog.V(2) {
			glog.Infof("Skipping profile %d", itemID)
		}
		return nil
	}

	// Read profile from disk
	//
	// Done here so that if we are resuming and only a few failed we only end up
	// reading a few things from disk rather than everything.
	sp := src.Profile{}
	err := files.JSONFileToInterface(
		files.GetPath(args.ItemTypeID, itemID),
		&sp,
	)
	if err != nil {
		glog.Errorf("Failed to load profile from JSON: %+v", err)
		return err
	}

	profile, err := createProfile(args, sp)
	if err != nil {
		glog.Errorf("Failed to createProfile %d : %+v", itemID, err)
		return err
	}

	if sp.Avatar.ContentURL != "" {
		glog.Infof("Processing avatar for %v", profile)
		err := processAvatar(sp, profile)
		// Avatar processing errors are not fatal, so don't return.
		if err != nil {
			glog.Errorf("Error processing avatar: %s", err)
		}
	}

	tx, err := h.GetTransaction()
	if err != nil {
		glog.Errorf("Failed to get transaction: %+v", err)
		return err
	}
	defer tx.Rollback()

	err = accounting.RecordImport(
		tx,
		args.OriginID,
		args.ItemTypeID,
		sp.ID,
		profile.Id,
	)
	if err != nil {
		glog.Errorf("Failed to recordImport: %+v", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		glog.Errorf("Failed to commit transaction: %+v", err)
		return err
	}

	audit.Create(
		args.SiteID,
		h.ItemTypes[h.ItemTypeProfile],
		profile.Id,
		profile.Id,
		sp.DateCreated,
		net.ParseIP(sp.IPAddress),
	)

	if glog.V(2) {
		glog.Infof("Successfully imported profile %d", itemID)
	}
	return nil
}

// createProfile puts a profile into the database via microcosm models
func createProfile(args conc.Args, sp src.Profile) (models.ProfileType, error) {

	u, status, err := models.GetUserByEmailAddress(sp.Email)
	if err != nil && status != http.StatusNotFound {
		glog.Errorf(
			"Failed to get existing user by email address: <%s> %+v",
			sp.Email,
			err,
		)
		return models.ProfileType{}, err
	}
	if status == http.StatusNotFound {
		glog.Infof("Creating new user for email address: %s", sp.Email)
		// User doesn't exist, so we should create it
		u = models.UserType{}
		u.Email = sp.Email
		u.Language = "en-GB"

		_, err := u.Insert()
		if err != nil {
			glog.Errorf("Failed to create user for profile: %+v", err)
			return models.ProfileType{}, err
		}
	} else {
		// User does exist, so we should check whether that user already has a
		// profile on this site and return that.
		profileID, status, err := models.GetProfileId(args.SiteID, u.ID)
		if err != nil && status != http.StatusNotFound {
			glog.Errorf("Failed to get existing profile for user: %+v", err)
			return models.ProfileType{}, err
		}
		// If profile ID exists, fetch full profile and return. Otherwise,
		// a new profile is created below.
		if status == http.StatusOK {
			profile, _, err := models.GetProfile(args.SiteID, profileID)
			if err != nil {
				glog.Errorf("Failed to retrieve existing profile %d: %s", profileID, err)
			}
			return profile, nil
		}
	}

	glog.Infof("Creating new profile: user: %d", u.ID)
	// We don't have a profile, but we do now have a user, so create the profile
	p := models.ProfileType{}
	p.SiteId = args.SiteID
	p.UserId = u.ID
	p.ProfileName = sp.Name
	p.Created = sp.DateCreated
	p.LastActive = sp.LastActive
	p.Visible = true
	p.StyleId = 1
	p.AvatarUrlNullable = sql.NullString{
		String: "/api/v1/files/66cca61feb8001cb71a9fb7062ff94c9d2543340",
		Valid:  true,
	}

	_, err = p.Import()
	if err != nil {
		glog.Errorf("Failed to create profile for imported user %d: %+v", sp.ID, err)
		return p, err
	}

	return p, nil
}

func processAvatar(sp src.Profile, profile models.ProfileType) error {

	// Create and associate profile avatar.
	// Use FileMetadata.Import which idempotently creates necessary row.
	fm := models.FileMetadataType{
		Created:     sp.Avatar.DateCreated,
		FileName:    sp.Avatar.Name,
		FileSize:    sp.Avatar.ContentSize,
		MimeType:    sp.Avatar.MimeType,
		Width:       sp.Avatar.Width,
		Height:      sp.Avatar.Height,
		AttachCount: int64(len(sp.Avatar.Associations)),
	}

	parts := strings.SplitN(sp.Avatar.ContentURL, ",", 2)
	if len(parts) != 2 {
		err := errors.New(fmt.Sprintf("Unexpected data URI, attachment %d\n", sp.Avatar.ID))
		glog.Error(err)
		return err
	}

	content, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		err = fmt.Errorf("Could not decode attachment %d: %s\n", sp.Avatar.ID, err)
		glog.Error(err)
		return err
	}
	fm.Content = content

	SHA1, err := h.Sha1(content)
	if err != nil {
		err = fmt.Errorf("Could not generate SHA-1 for attachment %d: %s\n", sp.Avatar.ID, err)
		glog.Error(err)
		return err
	}
	fm.FileHash = SHA1

	max := int64(1)<<32 - 1
	_, err = fm.Import(max, max)
	if err != nil {
		glog.Error(err)
		return err
	}

	// File metadata created, now create an attachment row.
	// Look up the author profile based on the old user ID.
	assoc := sp.Avatar.Associations[0]

	var assocItemTypeID int64
	switch assoc.OnType {
	case "profile":
		assocItemTypeID = 3
	case "user":
		assocItemTypeID = 3
	default:
		return fmt.Errorf("Unknown attachment association: %s\n", assoc.OnType)
	}

	at := models.AttachmentType{
		AttachmentMetaId: fm.AttachmentMetaId,
		ProfileId:        profile.Id,
		ItemTypeId:       assocItemTypeID,
		ItemId:           profile.Id,
		FileHash:         SHA1,
		FileName:         sp.Avatar.Name,
		Created:          sp.Avatar.DateCreated,
	}
	_, err = at.Import()
	if err != nil {
		glog.Errorf("Could not import attachment %d\n", sp.Avatar.ID)
		return err
	}

	// Now associate the attachment with the profile.
	profile.AvatarUrlNullable = sql.NullString{
		String: fmt.Sprintf("%s/%s", h.ApiTypeFile, fm.FileHash),
		Valid:  true,
	}
	profile.AvatarIdNullable = sql.NullInt64{
		Int64: at.AttachmentId,
		Valid: true,
	}
	if profile.SiteId == 0 {
		glog.Errorf("%d", profile.SiteId)

	}
	_, err = profile.Update()
	if err != nil {
		glog.Errorf("Failed to update profile: %+v", err)
		return err
	}

	return nil
}
