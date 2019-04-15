package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/camptocamp/terradb/internal/storage"
)

// API defines an API struct
type API struct {
	Address  string
	Port     string
	Username string
	Password string
}

type server struct {
	st storage.Storage
}

// StartServer starts the API server
func StartServer(cfg *API, st storage.Storage) {
	s := server{
		st: st,
	}

	router := mux.NewRouter().StrictSlash(true)

	apiRtr := router.PathPrefix("/v1/states").Subrouter()
	apiRtr.HandleFunc("/{name}", s.InsertState).Methods("POST")
	apiRtr.HandleFunc("/{name}", s.GetState).Methods("GET")
	apiRtr.HandleFunc("/{name}", s.RemoveState).Methods("DELETE")
	apiRtr.HandleFunc("/{name}", s.LockState).Methods("LOCK")
	apiRtr.HandleFunc("/{name}", s.UnlockState).Methods("UNLOCK")

	log.Infof("Listening on %s:%s", cfg.Address, cfg.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", cfg.Address, cfg.Port), router))
	return
}

func (s *server) HandleAPIRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
