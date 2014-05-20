package main

import (
	"database/sql"
	"encoding/json"
	exports "github.com/microcosm-cc/export-schemas/go/forum"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type UserType struct {
	Id      int64     `json:"userId"`
	Email   string    `json:"email"`
	Created time.Time `json:"created"`
	Banned  bool      `json:"banned,omitempty"`
}

const usersPath string = "users"

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

func LoadUsers(rootpath string, ownerId int64) (owner UserType, users []UserType, err error) {

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

	// Walk each path in the "users" directory.
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
		exUser := exports.User{}
		err = json.Unmarshal(bytes, &exUser)
		if err != nil {
			log.Print(err)
			continue
		}

		user := UserType{
			Email:   exUser.Email,
			Created: exUser.DateCreated,
			Banned:  exUser.Banned,
		}

		if exUser.Id == ownerId {
			owner = user
		} else {
			users = append(users, user)
		}
	}
	return
}

func StoreUser(db *sql.DB, user UserType) (userId int64, err error) {

	tx, err := db.Begin()
	if err != nil {
		return
	}
	err = tx.QueryRow(`INSERT INTO users (email, created, banned) VALUES ($1, $2, $3) RETURNING user_id;`, user.Email, user.Created, user.Banned).Scan(&userId)
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return
	}
	return
}
