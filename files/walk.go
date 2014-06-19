package files

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	src "github.com/microcosm-cc/export-schemas/go/forum"
)

// WalkExportTree walks an export subdirectory and adding valid paths to the map
// of all identifiers that we can extract from the subdirectories and filenames.
// i.e. for the path:
//   comments/321/321/1.json
// we expect the value:
//   [3213211]"comments/321/321/1.json"
// and so on for all paths within the directory structure
func WalkExportTree(rootPath string, itemTypeID int64) error {
	indexFile := path.Join(rootPath, getPathForItemType(itemTypeID), "index.json")
	if Exists(indexFile) {
		return loadIndex(rootPath, indexFile, itemTypeID)
	}

	return doWalk(rootPath, itemTypeID)
}

func doWalk(rootPath string, itemTypeID int64) error {
	glog.Info("Walking tree")
	// Define what should be done in the subsequent directory walk.
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			glog.Errorf("Walk error %+v", err)
			return err
		}

		if filepath.Ext(path) == ".json" {
			addPath(itemTypeID, path, 0)
		}

		return nil
	}

	// Walk each path in the directory.
	return filepath.Walk(
		path.Join(rootPath, getPathForItemType(itemTypeID)),
		walk,
	)
}

func loadIndex(rootPath string, indexFile string, itemTypeID int64) error {
	glog.Info("Loading index")
	dirIndex := src.DirIndex{}
	err := JSONFileToInterface(indexFile, &dirIndex)
	if err != nil {
		glog.Errorf("Index load error %+v", err)
		return err
	}

	for _, df := range dirIndex.Files {
		addPath(
			itemTypeID,
			path.Join(rootPath, strings.Replace(df.Path, `//`, `/`, -1)),
			df.ID,
		)
	}

	return nil
}
