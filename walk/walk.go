package walk

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func storeID(itemType string, path string, exportsMap map[int]string) {
	// Split path on itemType (e.g. "users")
	strID := strings.Split(path, itemType)[1]
	// Split file extension
	strID = strings.Split(strID, ".json")[0]
	// Replace path delimiters to extract ID.
	strID = strings.Replace(strID, "/", "", -1)

	// Put ID and associated file path in exportsMap.
	ID, err := strconv.Atoi(strID)
	if err != nil {
		log.Print(err)
	}
	exportsMap[ID] = path
}

// WalkExports walks an export subdirectory and builds a map of all identifiers
// that we can extract from the subdirectories and filenames.
// i.e. for the path:
//   comments/321/321/1.json
// we expect the value:
//   [3213211]"comments/321/321/1.json"
// and so on for all paths within the directory structure
func WalkExports(exportsPath string, itemType string) (exportsMap map[int]string, err error) {

	// Build a map of Item ID -> file path.
	exportsMap = make(map[int]string)

	// Define what should be done in the subsequent directory walk.
	walkFunc := func(walkpath string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return err
		}
		if filepath.Ext(walkpath) == ".json" {
			storeID(itemType, walkpath, exportsMap)
		}
		return err
	}

	// Walk each path in the directory.
	err = filepath.Walk(path.Join(exportsPath, itemType), walkFunc)
	return
}
