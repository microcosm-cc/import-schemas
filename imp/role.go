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

		// Get the new microcosmID
		microcosmID := accounting.GetNewID(
			args.OriginID,
			h.ItemTypes[h.ItemTypeMicrocosm],
			forumID,
		)
		if microcosmID == 0 {
			glog.Error(fmt.Errorf("Expected microcosm for %d", forumID))
			return []error{fmt.Errorf("Expected microcosm for %d", forumID)}
		}

		if len(srcForum.Moderators) == 0 && len(srcForum.Usergroups) == 0 {
			bar.Increment()
			continue
		}

		// Start with copying any usergroups
		var foundModsRole bool
		if len(srcForum.Usergroups) > 0 {
			// We need to copy all usergroups
			for _, oldRoleId := range oldRoleIDS {
				role, ok := roles[oldRoleId]
				if !ok {
					glog.Error(fmt.Errorf("Expected role for %d", oldRoleId))
					continue
				}

				// Update the role for this microcosm
				role.MicrocosmId = microcosmID

				// And override the ones that were defined by the forum
				var quickExit bool
				for _, usergroup := range srcForum.Usergroups {
					if oldRoleId == usergroup.ID {

						role.CanRead = usergroup.ForumPermissions.View
						role.CanReadOthers = usergroup.ForumPermissions.View
						role.CanCreate = usergroup.ForumPermissions.PostNew
						role.CanUpdate = usergroup.ForumPermissions.EditOthers
						role.CanDelete = usergroup.ForumPermissions.DeleteOthers
						role.CanCloseOwn = usergroup.ForumPermissions.CloseOwn
						role.CanOpenOwn = usergroup.ForumPermissions.OpenOwn

						if !role.CanRead && !role.CanReadOthers &&
							!role.CanCreate && !role.CanUpdate && !role.CanDelete &&
							!role.CanCloseOwn && !role.CanOpenOwn {
							// Everything is false, as our permissions are based on
							// whitelisting permissions, a full set of blacklists is
							// equivalent to doing nothing
							quickExit = true
						}
						break
					}
				}
				if quickExit {
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

				if role.IsModerator {
					foundModsRole = true
					// Add the profiles declared as members, and keep track of them
					// as we also need to add the moderators in the srcForum info
					modsAdded := make(map[int64]bool)
					for _, p := range role.Profiles {
						modsAdded[p.Id] = true
						_, err := p.Update(args.SiteID, role.Id)
						if err != nil {
							glog.Error(err)
							return []error{err}
						}
					}

					// Add the moderators
					for _, oldModID := range srcForum.Moderators {
						modID := accounting.GetNewID(
							args.OriginID,
							h.ItemTypes[h.ItemTypeProfile],
							oldModID.ID,
						)
						if modID == 0 {
							continue
						}
						if _, ok := modsAdded[modID]; !ok {
							rp := models.RoleProfileType{}
							rp.Id = modID
							_, err := rp.Update(args.SiteID, role.Id)
							if err != nil {
								glog.Error(err)
								return []error{err}
							}
						}
					}
				} else {
					// Just add the profiles that were already declared as members
					for _, p := range role.Profiles {
						_, err := p.Update(args.SiteID, role.Id)
						if err != nil {
							glog.Error(err)
							return []error{err}
						}
					}
				}
			}
		}

		// We have moderators that haven't yet been placed into a moderators
		// role. We'll create a moderator role and assign the people
		if !foundModsRole && len(srcForum.Moderators) > 0 {
			modRole := models.RoleType{}
			modRole.SiteId = args.SiteID
			modRole.MicrocosmId = microcosmID
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
				modID := accounting.GetNewID(
					args.OriginID,
					h.ItemTypes[h.ItemTypeProfile],
					oldModID.ID,
				)
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
