package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/deemkeen/stegodon/app"
	"github.com/deemkeen/stegodon/util"
)

func main() {
	// Parse command line flags
	versionFlag := flag.Bool("v", false, "Print version information")
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("stegodon v%s\n", util.GetVersion())
		os.Exit(0)
	}

	// Load configuration
	conf, err := util.ReadConf()
	if err != nil {
		log.Fatalln(err)
	}

	// Setup logging (journald if enabled, otherwise standard logging)
	util.SetupLogging(conf.Conf.WithJournald)

	log.Printf("stegodon v%s", util.GetVersion())
	log.Println("Configuration: ")
	log.Println(util.PrettyPrint(conf))

	// Start pprof server for profiling (if enabled)
	if conf.Conf.WithPprof {
		go func() {
			log.Println("pprof server listening on localhost:6060")
			log.Println("Access profiling at http://localhost:6060/debug/pprof/")
			if err := http.ListenAndServe("localhost:6060", nil); err != nil {
				log.Printf("pprof server error: %v", err)
			}
		}()
	}

	// Create and initialize the application
	application, err := app.New(conf)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	if err := application.Initialize(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start the application (blocks until shutdown signal)
	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
