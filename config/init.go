package config

import (
	"github.com/golang/glog"

	"github.com/microcosm-cc/goconfig"
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
