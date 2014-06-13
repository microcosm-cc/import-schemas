package main

import (
	"flag"

	// Included for the side effect of configuring the logger via flag.Parse()
	_ "github.com/golang/glog"

	// "github.com/microcosm-cc/microcosm/cache"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/config"
	"github.com/microcosm-cc/import-schemas/imp"
)

func main() {
	flag.Parse()

	h.InitDBConnection(h.DBConfig{
		Host:     config.DbHost,
		Port:     config.DbPort,
		Database: config.DbName,
		Username: config.DbUser,
		Password: config.DbPass,
	})

	// // If you want to use memcache to help speed things up... the cache misses
	// // may not make this as fast as you hope though
	// cache.InitCache("localhost", 11211)

	imp.Import()
}
