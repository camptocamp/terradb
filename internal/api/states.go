package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/camptocamp/terradb/internal/storage"
	"github.com/gorilla/mux"
	"github.com/hashicorp/terraform/state"
	"github.com/hashicorp/terraform/terraform"
)

func (s *server) InsertState(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	timestamp, ok := params["timestamp"]
	if !ok {
		timestamp = time.Now().Format("20060102150405")
	}

	source, ok := params["source"]
	if !ok {
		source = "direct"
	}

	var document terraform.State
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&document)
	if err != nil {
		err500(err, "failed to decode body", w)
		return
	}

	err = s.st.InsertState(document, timestamp, source, params["name"])
	if err != nil {
		err500(err, "failed to insert state", w)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

func (s *server) ListStates(w http.ResponseWriter, r *http.Request) {
	page, per_page, err := parsePagination(r)
	if err != nil {
		err500(err, "", w)
		return
	}

	coll, err := s.st.ListStates(page, per_page)
	if err != nil {
		err500(err, "failed to retrieve states", w)
		return
	}

	/*
		if len(coll.Data) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	*/

	data, err := json.Marshal(coll)
	if err != nil {
		err500(err, "failed to marshal states", w)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
	return
}

func (s *server) GetState(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	var serial int
	if v := r.URL.Query().Get("serial"); v != "" {
		var err error
		serial, err = strconv.Atoi(v)
		if err != nil {
			err500(err, "failed to parse serial", w)
			return
		}
	}

	document, err := s.st.GetState(params["name"], serial)
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

func (s *server) RemoveState(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	err := s.st.RemoveState(params["name"])
	if err != nil {
		err500(err, "failed to remove state", w)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

func (s *server) LockState(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	var currentLock, remoteLock *state.LockInfo

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		err500(err, "failed to read body", w)
		return
	}

	err = json.Unmarshal(body, &currentLock)
	if err != nil {
		err500(err, "failed to unmarshal lock", w)
		return
	}

	lock, err := s.st.GetLockStatus(params["name"])
	if err != nil {
		err500(err, "failed to get lock status", w)
		return
	}

	if lock != nil {
		remoteLock = lock.(*state.LockInfo)

		if currentLock.ID == remoteLock.ID {
			d, _ := json.Marshal(lock)
			w.WriteHeader(http.StatusLocked)
			w.Write(d)
			return
		}

		d, _ := json.Marshal(remoteLock)
		w.WriteHeader(http.StatusConflict)
		w.Write(d)
		return
	}

	err = s.st.LockState(params["name"], currentLock)
	if err != nil {
		err500(err, "failed to lock state", w)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

func (s *server) UnlockState(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	var lockData *state.LockInfo

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		err500(err, "failed to read body", w)
		return
	}

	err = json.Unmarshal(body, &lockData)
	if err != nil {
		err500(err, "failed to unmarshal lock", w)
		return
	}

	err = s.st.UnlockState(params["name"], lockData)
	if err != nil {
		err500(err, "failed to unlock state", w)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

func (s *server) ListStateSerials(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	page, per_page, err := parsePagination(r)
	if err != nil {
		err500(err, "", w)
		return
	}

	coll, err := s.st.ListStateSerials(params["name"], page, per_page)
	if err != nil {
		err500(err, "failed to retrieve state serials", w)
		return
	}

	data, err := json.Marshal(coll)
	if err != nil {
		err500(err, "failed to marshal state serials", w)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
	return
}

func parsePagination(r *http.Request) (page, per_page int, err error) {
	page = 1
	per_page = 100
	if v := r.URL.Query().Get("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil {
			return page, per_page, fmt.Errorf("failed to parse page: %v", err)
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		per_page, err = strconv.Atoi(v)
		if err != nil {
			return page, per_page, fmt.Errorf("failed to parse per_page: %v", err)
		}
	}
	return
}
