package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"github.com/golang/glog"

	"github.com/microcosm-cc/microcosm/cache"
	h "github.com/microcosm-cc/microcosm/helpers"

	"github.com/microcosm-cc/import-schemas/config"
	"github.com/microcosm-cc/import-schemas/imp"
)

var memprof = flag.String("memprof", "", "write memory profile to file")
var cpuprof = flag.String("cpuprof", "", "write cpu profile to file")

func main() {
	flag.Parse()

	// Go as fast as we can
	runtime.GOMAXPROCS(runtime.NumCPU())

	if *memprof != "" {
		// Reference time is used for formatting.
		// See http://golang.org/pkg/time for details.
		f, err := os.Create(*memprof)
		if err != nil {
			glog.Fatal(err)
		}

		// Catch SIGINT and write heap profile
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT)
		go func() {
			for sig := range c {
				glog.Warningf("Caught %v, stopping profiler and exiting..", sig)
				// Heap profiler is run on GC, so make sure it GCs before exiting.
				runtime.GC()
				pprof.WriteHeapProfile(f)
				f.Close()
				glog.Flush()
				os.Exit(1)
			}
		}()
	} else {
		// Catch closing signal and flush logs
		sigc := make(chan os.Signal, 1)
		signal.Notify(
			sigc,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT,
		)
		go func() {
			<-sigc
			glog.Flush()
			os.Exit(1)
		}()
	}

	if *cpuprof != "" {
		// Reference time is used for formatting.
		// See http://golang.org/pkg/time for details.
		f, err := os.Create(*cpuprof)
		if err != nil {
			glog.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

		// Catch closing signal and flush logs
		sigc := make(chan os.Signal, 1)
		signal.Notify(
			sigc,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT,
		)
		go func() {
			<-sigc
			pprof.StopCPUProfile()
			glog.Flush()
			os.Exit(1)
		}()
	}

	h.InitDBConnection(h.DBConfig{
		Host:     config.DbHost,
		Port:     config.DbPort,
		Database: config.DbName,
		Username: config.DbUser,
		Password: config.DbPass,
	})

	// If you want to use memcache to help speed things up... the cache misses
	// may not make this as fast as you hope though
	cache.InitCache("localhost", 11211)

	imp.Import()
}
