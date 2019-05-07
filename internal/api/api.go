package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/rs/cors"

	"github.com/camptocamp/terradb/internal/storage"
)

// API defines an API struct
type API struct {
	Address  string
	Port     string
	Username string
	Password string
	PageSize int
}

type server struct {
	st       storage.Storage
	pageSize int
	username string
	password string
}

// StartServer starts the API server
func StartServer(cfg *API, st storage.Storage) {
	s := server{
		st:       st,
		pageSize: cfg.PageSize,
		username: cfg.Username,
		password: cfg.Password,
	}

	router := mux.NewRouter().StrictSlash(true)

	router.Use(s.handleAPIRequest)

	apiRtr := router.PathPrefix("/v1").Subrouter()
	apiRtr.HandleFunc("/states", s.ListStates).Methods("GET")
	apiRtr.HandleFunc("/states/{name}", s.InsertState).Methods("POST")
	apiRtr.HandleFunc("/states/{name}", s.GetState).Methods("GET")
	apiRtr.HandleFunc("/states/{name}", s.RemoveState).Methods("DELETE")
	apiRtr.HandleFunc("/states/{name}", s.LockState).Methods("LOCK")
	apiRtr.HandleFunc("/states/{name}", s.UnlockState).Methods("UNLOCK")
	apiRtr.HandleFunc("/states/{name}/serials", s.ListStateSerials).Methods("GET")
	apiRtr.HandleFunc("/resources/{state}/{module}/{name}", s.GetResource).Methods("GET")
	apiRtr.HandleFunc("/resources/{state}/{name}", s.GetResource).Methods("GET")

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})

	handler := c.Handler(router)

	log.Infof("Listening on %s:%s", cfg.Address, cfg.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%s", cfg.Address, cfg.Port), handler))
	return
}

func (s *server) handleAPIRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		if !isAuthorized(r.Header.Get("Authorization"), s.username, s.password) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("401 - Not authorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func err500(err error, msg string, w http.ResponseWriter) {
	log.Errorf("%s: %s", msg, err)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(fmt.Sprintf("500 - Internal server error: %s", err)))
	return
}

func isAuthorized(authorizationHeader, username, password string) bool {
	if username == "" || password == "" {
		return true
	}

	s := strings.SplitN(authorizationHeader, " ", 2)
	if len(s) != 2 {
		return false
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return false
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return false
	}

	if pair[0] != username || pair[1] != password {
		return false
	}
	return true
}
