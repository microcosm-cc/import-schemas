package main

import (
	models "github.com/microcosm-cc/export-schemas/go/forum"
	"github.com/microcosm-cc/goconfig"

	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	configFile          = "config.toml"
	configExportSection = "export"
	usersPath           = "users"
)

func storeID(itemType string, path string, itemMap map[int]string) {
	// Split path on itemType (e.g. "users")
	strID := strings.Split(path, itemType)[1]
	// Split file extension
	strID = strings.Split(strID, ".json")[0]
	// Replace path delimiters to extract ID.
	strID = strings.Replace(strID, "/", "", -1)

	// Store ID and associated file path in itemMap.
	ID, err := strconv.Atoi(strID)
	if err != nil {
		log.Print(err)
	}
	itemMap[ID] = path
}

func main() {

	// Load config.
	config, err := goconfig.ReadConfigFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	rootpath, err := config.GetString(configExportSection, "rootpath")
	if err != nil {
		log.Print(err)
	}

	// Build a map of User ID -> file path.
	userMap := make(map[int]string)

	// Define what should be done in the subsequent directory walk.
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print(err)
			return err
		}
		if filepath.Ext(path) == ".json" {
			storeID(usersPath, path, userMap)
		}
		return nil
	}

	// Walk the each path in the "users" directory.
	filepath.Walk(path.Join(rootpath, usersPath), walkFunc)

	var keys []int
	for key, _ := range userMap {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	// Iterate map in order
	for _, ID := range keys {
		bytes, err := ioutil.ReadFile(userMap[ID])
		if err != nil {
			log.Printf("Error opening path: %d\n", ID)
			continue
		}
		user := models.User{}
		json.Unmarshal(bytes, &user)
	}
}
