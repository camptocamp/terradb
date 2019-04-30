package storage

import (
	"errors"
	"time"

	"github.com/hashicorp/terraform/state"
	"github.com/hashicorp/terraform/terraform"
)

// State is a Terraform state
type State terraform.State

// LockInfo is a State lock info
type LockInfo state.LockInfo

// Document associates a name and a state
type Document struct {
	Timestamp    string    `json:"-"`
	LastModified time.Time `json:"last_modified"`
	Name         string    `json:"name"`
	State        *State    `json:"state"`
}

// DocumentCollection is a collection of Document, with metadata
type DocumentCollection struct {
	Metadata []struct {
		Total int `json:"total"`
		Page  int `json:"page"`
	} `json:"metadata"`
	Data []*Document `json:"data"`
}

// ErrNoDocuments
var ErrNoDocuments = errors.New("No document found")

// Storage is an abstraction over database engines
type Storage interface {
	GetName() string
	ListStates(page_num, page_size int) (coll DocumentCollection, err error)
	GetState(name string, serial int) (state State, err error)
	InsertState(document State, timestamp, source, name string) (err error)
	RemoveState(name string) (err error)
	GetLockStatus(name string) (lockStatus LockInfo, err error)
	LockState(name string, lockData LockInfo) (err error)
	UnlockState(name string, lockData LockInfo) (err error)
	ListStateSerials(name string, page_num, page_size int) (coll DocumentCollection, err error)
}
