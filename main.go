package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"

	"github.com/camptocamp/terradb/internal/api"
	"github.com/camptocamp/terradb/internal/storage"
)

var opts struct {
	Version bool `short:"V" long:"version" description:"Display version."`
	MongoDB struct {
		URL      string `long:"mongodb-url" description:"MongoDB URL" env:"MONGODB_URL"`
		Username string `long:"mongodb-username" description:"MongoDB Username" env:"MONGODB_USERNAME"`
		Password string `long:"mongodb-password" description:"MongoDB Password" env:"MONGODB_PASSWORD"`
	} `group:"MongoDB options"`
	API struct {
		Address string `long:"api-address" description:"Address on to bind the API server" env:"API_ADDRESS" default:"127.0.0.1"`
		Port    string `long:"api-port" description:"Port on to listen" env:"API_PORT" default:"8080"`
	} `group:"API server options"`
}

// VERSION is TerraDB's version number
var VERSION = "undefined"

func main() {
	var st storage.Storage

	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()
	if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
		os.Exit(0)
	}
	if err != nil {
		log.Fatal(err)
	}

	if opts.Version {
		fmt.Printf("TerraDB v%v\n", VERSION)
		os.Exit(0)
	}

	st, err = storage.NewMongoDB(&storage.MongoDBConfig{
		URL:      opts.MongoDB.URL,
		Username: opts.MongoDB.Username,
		Password: opts.MongoDB.Password,
	})
	if err != nil {
		log.Fatalf("failed to setup storage: %s", err)
	}

	api.StartServer(&api.API{
		Address: opts.API.Address,
		Port:    opts.API.Port,
	}, st)
}
