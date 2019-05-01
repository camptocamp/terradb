package api

import (
	"encoding/json"
	"net/http"

	"github.com/camptocamp/terradb/internal/storage"
	"github.com/gorilla/mux"
)

func (s *server) GetResource(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	state := params["state"]
	module, ok := params["module"]
	if !ok {
		module = "root"
	}
	name := params["name"]

	document, err := s.st.GetResource(state, module, name)
	if err == storage.ErrNoDocuments {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		err500(err, "failed to retrieve latest state", w)
		return
	}

	data, err := json.Marshal(document)
	if err != nil {
		err500(err, "failed to marshal state", w)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
	return
}
