package main

import (
	"log"

	_ "github.com/lib/pq"

	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/config"
	"github.com/microcosm-cc/import-schemas/imp"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	h.InitDBConnection(h.DBConfig{
		Host:     config.DbHost,
		Port:     config.DbPort,
		Database: config.DbName,
		Username: config.DbUser,
		Password: config.DbPass,
	})

	imp.Import()
}
