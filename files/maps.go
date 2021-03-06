package files

import (
	// "path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"

	h "github.com/microcosm-cc/microcosm/helpers"
)

var (
	attachmentIDsToPaths       map[int64]string
	attachmentIDsToPathsLock   sync.Mutex
	commentIDsToPaths          map[int64]string
	commentIDsToPathsLock      sync.Mutex
	conversationIDsToPaths     map[int64]string
	conversationIDsToPathsLock sync.Mutex
	huddleIDsToPaths           map[int64]string
	huddleIDsToPathsLock       sync.Mutex
	microcosmIDsToPaths        map[int64]string
	microcosmIDsToPathsLock    sync.Mutex
	profileIDsToPaths          map[int64]string
	profileIDsToPathsLock      sync.Mutex
	roleIDsToPaths             map[int64]string
	roleIDsToPathsLock         sync.Mutex
	watcherIDsToPaths          map[int64]string
	watcherIDsToPathsLock      sync.Mutex
)

// Int64Slice attaches the methods of Interface to []int64, sorting in
// increasing order.
type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// GetIDs returns all of the IDs for a given item type as discovered by the walk
// of the exported files. The IDs are returned sorted in ascending order.
func GetIDs(itemTypeID int64) []int64 {
	var keys []int64

	switch itemTypeID {
	case h.ItemTypes[h.ItemTypeAttachment]:
		attachmentIDsToPathsLock.Lock()
		for key := range attachmentIDsToPaths {
			keys = append(keys, key)
		}
		attachmentIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeComment]:
		commentIDsToPathsLock.Lock()
		for key := range commentIDsToPaths {
			keys = append(keys, key)
		}
		commentIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeConversation]:
		conversationIDsToPathsLock.Lock()
		for key := range conversationIDsToPaths {
			keys = append(keys, key)
		}
		conversationIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeHuddle]:
		huddleIDsToPathsLock.Lock()
		for key := range huddleIDsToPaths {
			keys = append(keys, key)
		}
		huddleIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeMicrocosm]:
		microcosmIDsToPathsLock.Lock()
		for key := range microcosmIDsToPaths {
			keys = append(keys, key)
		}
		microcosmIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeProfile]:
		profileIDsToPathsLock.Lock()
		for key := range profileIDsToPaths {
			keys = append(keys, key)
		}
		profileIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeRole]:
		roleIDsToPathsLock.Lock()
		for key := range roleIDsToPaths {
			keys = append(keys, key)
		}
		roleIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeWatcher]:
		watcherIDsToPathsLock.Lock()
		for key := range watcherIDsToPaths {
			keys = append(keys, key)
		}
		watcherIDsToPathsLock.Unlock()

	default:
		glog.Fatal("Not yet implemented")
	}

	sort.Sort(Int64Slice(keys))

	return keys
}

func addPath(itemTypeID int64, name string, id int64) {
	if id == 0 {

		// path will be of the format:
		//   "users/321/231/1.json"
		// We need to get to:
		//   [3212311] = "users/321/231/1.json"
		// And we do this by stripping off the path prefix which is determined by
		// the itemTypeID
		filePath := strings.Split(name, getPathForItemType(itemTypeID))[1]

		// Removing the file extension
		filePath = strings.Split(filePath, ".json")[0]

		// Removing path delimiters
		filePath = strings.Replace(filePath, "/", "", -1)

		// And converting it to an int64
		var err error
		id, err = strconv.ParseInt(filePath, 10, 64)
		if err != nil {
			glog.Errorf("Failed to parseInt %s %+v", filePath, err)
			return
		}
	}

	// Finally we add the path to the approprate map
	switch itemTypeID {

	case h.ItemTypes[h.ItemTypeAttachment]:
		attachmentIDsToPathsLock.Lock()
		attachmentIDsToPaths[id] = name
		attachmentIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeComment]:
		commentIDsToPathsLock.Lock()
		commentIDsToPaths[id] = name
		commentIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeConversation]:
		conversationIDsToPathsLock.Lock()
		conversationIDsToPaths[id] = name
		conversationIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeHuddle]:
		huddleIDsToPathsLock.Lock()
		huddleIDsToPaths[id] = name
		huddleIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeMicrocosm]:
		microcosmIDsToPathsLock.Lock()
		microcosmIDsToPaths[id] = name
		microcosmIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeProfile]:
		profileIDsToPathsLock.Lock()
		profileIDsToPaths[id] = name
		profileIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeRole]:
		roleIDsToPathsLock.Lock()
		roleIDsToPaths[id] = name
		roleIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeWatcher]:
		watcherIDsToPathsLock.Lock()
		watcherIDsToPaths[id] = name
		watcherIDsToPathsLock.Unlock()

	default:
		glog.Fatal("Not yet implemented")
	}
}

// GetPath returns the path to a .json file for a given itemTypeID and itemID
func GetPath(itemTypeID int64, itemID int64) string {
	var (
		name string
		ok   bool
	)

	switch itemTypeID {

	case h.ItemTypes[h.ItemTypeAttachment]:
		attachmentIDsToPathsLock.Lock()
		name, ok = attachmentIDsToPaths[itemID]
		attachmentIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeComment]:
		commentIDsToPathsLock.Lock()
		name, ok = commentIDsToPaths[itemID]
		commentIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeConversation]:
		conversationIDsToPathsLock.Lock()
		name, ok = conversationIDsToPaths[itemID]
		conversationIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeHuddle]:
		huddleIDsToPathsLock.Lock()
		name, ok = huddleIDsToPaths[itemID]
		huddleIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeMicrocosm]:
		microcosmIDsToPathsLock.Lock()
		name, ok = microcosmIDsToPaths[itemID]
		microcosmIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeProfile]:
		profileIDsToPathsLock.Lock()
		name, ok = profileIDsToPaths[itemID]
		profileIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeRole]:
		roleIDsToPathsLock.Lock()
		name, ok = roleIDsToPaths[itemID]
		roleIDsToPathsLock.Unlock()

	case h.ItemTypes[h.ItemTypeWatcher]:
		watcherIDsToPathsLock.Lock()
		name, ok = watcherIDsToPaths[itemID]
		watcherIDsToPathsLock.Unlock()

	default:
		glog.Fatal("Not yet implemented")
	}

	if !ok {
		glog.Errorf(
			"File did not exist for itemTypeID %d and itemID %d",
			itemTypeID,
			itemID,
		)
		return ""
	}

	return name
}
