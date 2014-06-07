package files

import (
	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
	h "github.com/microcosm-cc/microcosm/helpers"
)

func getPathForItemType(itemTypeID int64) string {
	var path string

	switch itemTypeID {

	case h.ItemTypes[h.ItemTypeProfile]:
		path = src.ProfilesPath

	case h.ItemTypes[h.ItemTypeAttachment]:
		path = src.AttachmentsPath

	case h.ItemTypes[h.ItemTypeComment]:
		path = src.CommentsPath

	case h.ItemTypes[h.ItemTypeConversation]:
		path = src.ConversationsPath

	case h.ItemTypes[h.ItemTypeWatcher]:
		path = src.ForumsPath

	case h.ItemTypes[h.ItemTypeHuddle]:
		path = src.MessagesPath

	case h.ItemTypes[h.ItemTypeMicrocosm]:
		path = src.ForumsPath

	case h.ItemTypes[h.ItemTypeRole]:
		path = src.RolesPath

	default:
		glog.Fatal("Not yet implemented")
	}

	return path
}
