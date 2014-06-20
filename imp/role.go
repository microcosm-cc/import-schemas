package imp

import (
	"fmt"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/conc"
	"github.com/microcosm-cc/import-schemas/files"
)

var (
	forumMods       = make(map[int64][]int64)
	forumUsergroups = make(map[int64][]int64)
)

func importRoles(args conc.Args, gophers int) []error {

	args.ItemTypeID = h.ItemTypes[h.ItemTypeRole]

	// Loop forums
	//		If they have a single moderator, change the forum so that the single
	//			moderator is the owner.
	//		Read custom usergroups, define a list of all roles that are custom
	for _, forumID := range files.GetIDs(h.ItemTypes[h.ItemTypeMicrocosm]) {
		srcForum := src.Forum{}
		err := files.JSONFileToInterface(
			files.GetPath(h.ItemTypes[h.ItemTypeMicrocosm], forumID),
			&srcForum,
		)
		if err != nil {
			glog.Errorf("Failed to load forum from JSON: %+v", err)
			return []error{err}
		}

		for _, modID := range srcForum.Moderators {
			forumMods[srcForum.ID] = append(forumMods[srcForum.ID], modID.ID)
		}

		for _, group := range srcForum.Usergroups {
			forumUsergroups[srcForum.ID] = append(forumMods[srcForum.ID], group.ID)
		}
	}

	// Load knowledge of roles
	err := files.WalkExportTree(args.RootPath, args.ItemTypeID)
	if err != nil {
		exitWithError(err, []error{})
	}

	// Loop roles
	//		Build a list of all roles that were not forum custom... these *may*
	//			be our defaults, review this list
	defaultRoles := []int64{}
	for _, roleID := range files.GetIDs(args.ItemTypeID) {
		found := false
		for _, forumID := range files.GetIDs(h.ItemTypes[h.ItemTypeMicrocosm]) {
			if forumGroups, ok := forumUsergroups[forumID]; ok {
				for _, forumGroup := range forumGroups {
					if forumGroup == roleID {
						found = true
					}
				}
			}
		}
		if !found {
			defaultRoles = append(defaultRoles, roleID)
		}
	}
	fmt.Println(defaultRoles)

	return []error{}
}

func importRole(args conc.Args, itemID int64) error {
	return nil
}
