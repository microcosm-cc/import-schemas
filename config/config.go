package config

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
