package files

import (
	"log"

	h "github.com/microcosm-cc/microcosm/helpers"
)

func getPathForItemType(itemTypeID int64) string {
	var path string

	switch itemTypeID {

	case h.ItemTypes[h.ItemTypeProfile]:
		path = "users/"

	case h.ItemTypes[h.ItemTypeAttachment]:
		path = "attachments/"

	case h.ItemTypes[h.ItemTypeComment]:
		path = "comments/"

	case h.ItemTypes[h.ItemTypeConversation]:
		path = "conversations/"

	case h.ItemTypes[h.ItemTypeWatcher]:
		path = "follows/"

	case h.ItemTypes[h.ItemTypeHuddle]:
		path = "messages/"

	case h.ItemTypes[h.ItemTypeMicrocosm]:
		path = "forums/"

	case h.ItemTypes[h.ItemTypeRole]:
		path = "usergroups/"

	default:
		log.Fatal("Not yet implemented")
	}

	return path
}
