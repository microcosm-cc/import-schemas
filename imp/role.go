package imp

import (
	"fmt"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
	"github.com/microcosm-cc/microcosm/models"

	"github.com/microcosm-cc/import-schemas/accounting"
	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

var (
	// forumMods       = make(map[int64][]int64)
	// forumUsergroups = make(map[int64][]int64)

	roles        = make(map[int64]models.RoleType)
	defaultRoles = make(map[int64]bool)
)

func importRoles(args conc.Args, gophers int) []error {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeRole]

	// Roles are complex, in that there are default roles for the entire site in
	// addition to custom roles on specific microcosms. Role membership also
	// requires us to ensure that profiles are added either explicitly or via
	// a set of criteria.
	//
	// To make sense of roles we are going to:
	//   1. Discover all roles and load them locally into a slice of fully
	//        constructed roles
	//   2. Store them as the default roles
	//   3. Loop through all microcosms and look for ones that have role
	//        overrides
	//   4. For the one that have overrides, loop through all default roles and
	//        store them against a microcosm, opting for the override whenever
	//        that makes sense... and as our permissions are whitelist based, we
	//        can skip roles in which the permission set is all false/empty

	// 1.1 Load knowledge of roles
	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, []error{})
	}

	// 1.2 Load roles into a slice of fully constructed roles
	oldRoleIDS := files.GetIDs(args.ItemTypeID)

	for _, oldRoleId := range oldRoleIDS {
		srcRole := src.Role{}
		err := files.JSONFileToInterface(
			files.GetPath(args.ItemTypeID, oldRoleId),
			&srcRole,
		)
		if err != nil {
			glog.Errorf("Failed to load role from JSON: %+v", err)
			return []error{err}
		}

		role := models.RoleType{}
		role.Id = 0
		role.SiteId = args.SiteID
		role.Title = srcRole.Name
		role.IsBanned = srcRole.Banned
		role.IsModerator = srcRole.Moderator
		role.IncludeGuests = srcRole.IncludeGuests
		role.IncludeUsers = srcRole.IncludeRegistered

		if !role.IsBanned {
			role.CanRead = srcRole.ForumPermissions.View
			role.CanReadOthers = srcRole.ForumPermissions.View
			role.CanCreate = srcRole.ForumPermissions.PostNew
			role.CanUpdate = srcRole.ForumPermissions.EditOthers
			role.CanDelete = srcRole.ForumPermissions.DeleteOthers
			role.CanCloseOwn = srcRole.ForumPermissions.CloseOwn
			role.CanOpenOwn = srcRole.ForumPermissions.OpenOwn
		}

		role.Meta.Created = time.Now()
		role.Meta.CreatedById = args.SiteOwnerProfileID

		for _, u := range srcRole.Users {

			profileId := accounting.GetNewID(
				args.OriginID,
				h.ItemTypes[h.ItemTypeProfile],
				u.ID,
			)
			if profileId == 0 {
				continue
			}

			role.Profiles = append(
				role.Profiles,
				models.RoleProfileType{Id: profileId},
			)
		}

		for _, oc := range srcRole.Criteria {

			nc := models.RoleCriterionType{}

			nc.OrGroup = oc.OrGroup

			switch oc.Key {
			case "id", "profileName", "gender", "itemCount",
				"commentCount", "created", "isBanned":
				nc.ProfileColumn = oc.Key
			default:
				nc.AttrKey = oc.Key
			}

			nc.Predicate = oc.Predicate
			nc.Value = oc.Value

			role.Criteria = append(role.Criteria, nc)
		}

		roles[oldRoleId] = role

		if srcRole.DefaultRole {
			defaultRoles[srcRole.ID] = true
		}
	}

	// 2 store them as default roles
	fmt.Println("Importing default roles...")
	glog.Info("Importing default roles...")

	bar := pb.StartNew(len(oldRoleIDS))
	for _, oldRoleId := range oldRoleIDS {

		// Skip usergroups that people can apply to be a member of, these are
		// not site-wide default roles
		if _, ok := defaultRoles[oldRoleId]; !ok {
			bar.Increment()
			continue
		}

		role, ok := roles[oldRoleId]
		if !ok {
			glog.Error(fmt.Errorf("Expected role for %d", oldRoleId))
			continue
		}

		_, err := role.Insert(args.SiteID, args.SiteOwnerProfileID)
		if err != nil {
			glog.Errorf("%s %+v", err, role)
			return []error{err}
		}

		for _, c := range role.Criteria {
			_, err := c.Insert(role.Id)
			if err != nil {
				glog.Error(err)
				return []error{err}
			}
		}

		for _, p := range role.Profiles {
			_, err := p.Update(args.SiteID, role.Id)
			if err != nil {
				glog.Error(err)
				return []error{err}
			}
		}

		tx, err := h.GetTransaction()
		if err != nil {
			glog.Error(err)
			return []error{err}
		}
		defer tx.Rollback()

		err = accounting.RecordImport(
			tx,
			args.OriginID,
			args.ItemTypeID,
			oldRoleId,
			role.Id,
		)
		if err != nil {
			glog.Error(err)
			return []error{err}
		}

		err = tx.Commit()
		if err != nil {
			glog.Error(err)
			return []error{err}
		}

		if glog.V(2) {
			glog.Infof("Successfully imported role %d", oldRoleId)
		}
		bar.Increment()
	}
	bar.Finish()

	// 3 Import forum custom roles
	fmt.Println("Importing custom (microcosm specific) roles...")
	glog.Info("Importing custom (microcosm specific) roles...")

	// // Loop roles
	// //		Build a list of all roles that were not forum custom... these *may*
	// //			be our defaults, review this list
	// defaultRoles := []int64{}
	// for _, roleID := range files.GetIDs(args.ItemTypeID) {
	// 	found := false
	// 	for _, forumID := range files.GetIDs(h.ItemTypes[h.ItemTypeMicrocosm]) {
	// 		if forumGroups, ok := forumUsergroups[forumID]; ok {
	// 			for _, forumGroup := range forumGroups {
	// 				if forumGroup == roleID {
	// 					found = true
	// 				}
	// 			}
	// 		}
	// 	}
	// 	if !found {
	// 		defaultRoles = append(defaultRoles, roleID)
	// 	}
	// }
	// fmt.Println(defaultRoles)

	// Loop forums
	//		If they have a single moderator, change the forum so that the single
	//			moderator is the owner.
	//		Read custom usergroups, define a list of all roles that are custom
	oldForumIDs := files.GetIDs(h.ItemTypes[h.ItemTypeMicrocosm])
	bar = pb.StartNew(len(oldForumIDs))
	for _, forumID := range oldForumIDs {
		srcForum := src.Forum{}
		err := files.JSONFileToInterface(
			files.GetPath(h.ItemTypes[h.ItemTypeMicrocosm], forumID),
			&srcForum,
		)
		if err != nil {
			glog.Errorf("Failed to load forum from JSON: %+v", err)
			return []error{err}
		}

		// No custom usergroups, but we do have moderators... so we'll add a
		// moderator role and assign the people
		if len(srcForum.Moderators) > 0 && len(srcForum.Usergroups) == 0 {
			modRole := models.RoleType{}
			modRole.SiteId = args.SiteID
			modRole.MicrocosmId = accounting.GetNewID(
				args.OriginID,
				h.ItemTypes[h.ItemTypeMicrocosm],
				forumID,
			)
			if modRole.MicrocosmId == 0 {
				continue
			}
			modRole.Title = "Moderators"
			modRole.IsModerator = true
			modRole.CanRead = true
			modRole.CanReadOthers = true
			modRole.CanCreate = true
			modRole.CanUpdate = true
			modRole.CanDelete = true
			modRole.CanCloseOwn = true
			modRole.CanOpenOwn = true

			modRole.Meta.Created = time.Now()
			modRole.Meta.CreatedById = args.SiteOwnerProfileID

			_, err := modRole.Insert(args.SiteID, args.SiteOwnerProfileID)
			if err != nil {
				glog.Error(err)
				return []error{err}
			}

			for _, oldModID := range srcForum.Moderators {
				modID := accounting.GetNewID(args.OriginID, h.ItemTypes[h.ItemTypeProfile], oldModID.ID)
				if modID == 0 {
					continue
				}
				rp := models.RoleProfileType{}
				rp.Id = modID
				_, err := rp.Update(args.SiteID, modRole.Id)
				if err != nil {
					glog.Error(err)
					return []error{err}
				}
			}
		}

		// TODO: Add all custom roles, if there is a moderator role AND
		// moderators, then add those. If there isn't a moderator role and there
		// are moderators, create a new role.
		//
		// for _, group := range srcForum.Usergroups {
		// 	forumUsergroups[srcForum.ID] = append(forumMods[srcForum.ID], group.ID)
		// }
		bar.Increment()
	}
	bar.Finish()

	return []error{}
}

func importRole(args conc.Args, itemID int64) error {
	return nil
}
