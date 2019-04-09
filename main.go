package main

import (
	log "github.com/sirupsen/logrus"

	"github.com/camptocamp/terradb/internal/api"
	"github.com/camptocamp/terradb/internal/storage"
)

func main() {
	var st storage.Storage

	st, err := storage.NewMongoDB(&storage.MongoDBConfig{
		URL:      "mongodb://localhost:27017",
		Username: "root",
		Password: "root",
	})
	if err != nil {
		log.Fatalf("failed to setup storage: %s", err)
	}

	api.StartServer(&api.API{
		Address: "0.0.0.0",
		Port:    "8080",
	}, st)

	/*
		log.Info(st.GetName())

		rawDocument := `{"id": "1111", "foo": "bar"}`

		var structuredDocument interface{}
		json.Unmarshal([]byte(rawDocument), &structuredDocument)

		err = st.InsertTFState(structuredDocument, time.Now(), "direct")
		if err != nil {
			log.Fatalf("failed to insert document: %s", err)
		}
	*/
}
