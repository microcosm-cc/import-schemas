package main

import (
	"flag"

	_ "github.com/golang/glog"
	_ "github.com/lib/pq"

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
