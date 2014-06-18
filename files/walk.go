package files

import (
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"
)

var rootpath string

// WalkExportTree walks an export subdirectory and adding valid paths to the map
// of all identifiers that we can extract from the subdirectories and filenames.
// i.e. for the path:
//   comments/321/321/1.json
// we expect the value:
//   [3213211]"comments/321/321/1.json"
// and so on for all paths within the directory structure
func WalkExportTree(configRootPath string, itemTypeID int64) error {

	if rootpath == "" {
		rootpath = configRootPath
	}

	root, err := filepath.Abs(rootpath)
	if err != nil {
		glog.Errorf("Walk error %+v", err)
		return err
	}

	name := path.Join(root, getPathForItemType(itemTypeID))
	glog.Infof("Walking %s", name)

	// Walk each path in the directory.
	//
	// NOTE: We no longer use path.Walk as it sorts every directory by name and
	// this adds a considerable overhead when walking ~2,500 directories of
	// 2,000 items and artificially pushes up RAM use higher than we would like
	// it to be. Instead... we wrote our own version and skip the sorting as the
	// map utilities do that for us.
	return processDirectory(root, name, itemTypeID)
}

func processDirectory(root string, dirname string, itemTypeID int64) error {
	f, err := os.Open(dirname)
	if err != nil {
		glog.Errorf("Walk error %+v", err)
		return err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return err
	}

	for _, fi := range list {
		name := path.Join(dirname, fi.Name())

		if fi.IsDir() {
			err = processDirectory(root, name, itemTypeID)
			if err != nil {
				glog.Errorf("Walk error %+v", err)
				return err
			}
			continue
		}

		if path.Ext(name) == ".json" {
			addPath(itemTypeID, name[len(root):])
		}
	}

	return nil
}
