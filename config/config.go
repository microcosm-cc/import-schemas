package config

import (
	"github.com/microcosm-cc/goconfig"
	"log"
)

const (
	configFile            = "config.toml"
	configExportSection   = "export"
	configSiteSection     = "site"
	configDatabaseSection = "database"
)

var (
	DbHost           string
	DbPort           int64
	DbName           string
	DbUser           string
	DbPass           string
	SiteName         string
	SiteDesc         string
	SiteSubdomainKey string
	SiteOwnerId      int64
	Rootpath         string
)

func init() {
	var err error

	conf, err := goconfig.ReadConfigFile(configFile)

	// Database config.
	DbHost, err = conf.GetString(configDatabaseSection, "host")
	DbPort, err = conf.GetInt64(configDatabaseSection, "port")
	DbName, err = conf.GetString(configDatabaseSection, "database")
	DbUser, err = conf.GetString(configDatabaseSection, "username")
	DbPass, err = conf.GetString(configDatabaseSection, "password")

	// Site config.
	SiteName, err = conf.GetString(configSiteSection, "name")
	SiteDesc, err = conf.GetString(configSiteSection, "description")
	SiteSubdomainKey, err = conf.GetString(configSiteSection, "subdomain_key")
	SiteOwnerId, err = conf.GetInt64(configSiteSection, "owner_id")

	// Export config.
	Rootpath, err = conf.GetString(configExportSection, "rootpath")

	if err != nil {
		log.Fatal(err)
	}
}
