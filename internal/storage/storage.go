package storage

import (
	"errors"
	"time"

	"github.com/hashicorp/terraform/terraform"
)

// State is a Terraform state
type State terraform.State

// Document associates a name and a state
type Document struct {
	Timestamp    string    `json:"-"`
	LastModified time.Time `json:"last_modified"`
	Name         string    `json:"name"`
	State        *State    `json:"state"`
	Locked       bool      `json:"locked"`
	LockInfo     LockInfo  `json:"lock"`
}

// DocumentCollection is a collection of Document, with metadata
type DocumentCollection struct {
	Metadata []struct {
		Total int `json:"total"`
		Page  int `json:"page"`
	} `json:"metadata"`
	Data []*Document `json:"data"`
}

// LockInfo stores lock metadata.
//
// Copied from Terraform's source code
// to add missing json tags to structure fields
type LockInfo struct {
	// Unique ID for the lock. NewLockInfo provides a random ID, but this may
	// be overridden by the lock implementation. The final value if ID will be
	// returned by the call to Lock.
	ID string `json:"id"`

	// Terraform operation, provided by the caller.
	Operation string `json:"operation"`

	// Extra information to store with the lock, provided by the caller.
	Info string `json:"info"`

	// user@hostname when available
	Who string `json:"who"`

	// Terraform version
	Version string `json:"version"`

	// Time that the lock was taken.
	Created time.Time `json:"created"`

	// Path to the state file when applicable. Set by the Lock implementation.
	Path string `json:"path"`
}

// LockInfoDocument represents a stored LockInfo document
type LockInfoDocument struct {
	Name string   `json:"name"`
	Lock LockInfo `json:"lock"`
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
