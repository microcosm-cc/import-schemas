package files

import (
	"log"
	"os"
	"path"
	"path/filepath"
)

// WalkExportTree walks an export subdirectory and adding valid paths to the map
// of all identifiers that we can extract from the subdirectories and filenames.
// i.e. for the path:
//   comments/321/321/1.json
// we expect the value:
//   [3213211]"comments/321/321/1.json"
// and so on for all paths within the directory structure
func WalkExportTree(rootPath string, itemTypeID int64) error {
	// Define what should be done in the subsequent directory walk.
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return err
		}
		if filepath.Ext(path) == ".json" {
			addPath(itemTypeID, path)
		}
		return err
	}

	// Walk each path in the directory.
	return filepath.Walk(
		path.Join(rootPath, getPathForItemType(itemTypeID)),
		walk,
	)
}
