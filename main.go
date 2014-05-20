package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/microcosm-cc/import-schemas/config"
	"log"
)

func main() {
	connString := fmt.Sprintf("user=%s dbname=%s host=%s port=%d password=%s sslmode=disable",
		config.DbUser, config.DbName, config.DbHost, config.DbPort, config.DbPass)
	log.Print(connString)
	db, err := sql.Open("postgres", connString)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// Create a single user entry corresponding to the site owner.
	owner, _, err := LoadUsers(config.Rootpath, config.SiteOwnerId)
	log.Print(owner)
	id, err := StoreUser(db, owner)
	log.Print(id)
	log.Print(err)
	// Then, use create_owned_site which will create the site and owner's profile.
	// Then, store the rest of the users, creating profiles for each of them.
}
