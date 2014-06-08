package main

import (
	"flag"

	// Included for the side effect of configuring the logger via flag.Parse()
	_ "github.com/golang/glog"

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

	imp.Import()
}
