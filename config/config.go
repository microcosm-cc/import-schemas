package config

import (
	"github.com/golang/glog"

	"github.com/microcosm-cc/goconfig"
)

const (
	configFile            = "config.toml"
	configExportSection   = "export"
	configSiteSection     = "site"
	configDatabaseSection = "database"
)

var (
	// DbHost contains the name of the server,
	// i.e. 'localhost' or 'sql.dev.microcosm.cc'
	DbHost string

	// DbPort contains the port number that PostgreSQL is reachable on,
	// usually 5432
	DbPort int64

	// DbName contains the name of the database within PostgreSQL,
	// usually 'microcosm'
	DbName string

	// DbUser contains the name of the database user within the connection
	// string
	DbUser string

	// DbPass contains the name password to use when authenticating with
	// PostgreSQL
	DbPass string

	// SiteName contains the short name of the site we are importing
	SiteName string

	// SiteDesc contains the 2 sentence description of a site
	SiteDesc string

	// SiteSubdomainKey contains the Microcosm key that will be used in:
	// <SiteSubdomainKey>.microco.sm . It should all be lowercase.
	SiteSubdomainKey string

	// SiteOwnerID is the identifier for the user in the exported data that will
	// own the newly created site. For vBulletin systems this is usually userid
	// 1.
	SiteOwnerID int64

	// Rootpath is the relative or absolute path to the directory that contains
	// the exported data that we are importing
	Rootpath string
)

func init() {
	conf, err := goconfig.ReadConfigFile(configFile)
	if err != nil {
		glog.Fatal(err)
	}

	// Database config.
	DbHost, err = conf.GetString(configDatabaseSection, "host")
	if err != nil {
		glog.Fatal(err)
	}

	DbPort, err = conf.GetInt64(configDatabaseSection, "port")
	if err != nil {
		glog.Fatal(err)
	}

	DbName, err = conf.GetString(configDatabaseSection, "database")
	if err != nil {
		glog.Fatal(err)
	}

	DbUser, err = conf.GetString(configDatabaseSection, "username")
	if err != nil {
		glog.Fatal(err)
	}

	DbPass, err = conf.GetString(configDatabaseSection, "password")
	if err != nil {
		glog.Fatal(err)
	}

	// Site config.
	SiteName, err = conf.GetString(configSiteSection, "name")
	if err != nil {
		glog.Fatal(err)
	}

	SiteDesc, err = conf.GetString(configSiteSection, "description")
	if err != nil {
		glog.Fatal(err)
	}

	SiteSubdomainKey, err = conf.GetString(configSiteSection, "subdomain_key")
	if err != nil {
		glog.Fatal(err)
	}

	SiteOwnerID, err = conf.GetInt64(configSiteSection, "owner_id")
	if err != nil {
		glog.Fatal(err)
	}

	// Export config.
	Rootpath, err = conf.GetString(configExportSection, "rootpath")
	if err != nil {
		glog.Fatal(err)
	}
}
