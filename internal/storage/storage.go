package storage

import (
	"errors"
	"time"

	"github.com/hashicorp/terraform/terraform"
)

// State is a Terraform state
type State struct {
	LastModified time.Time `json:"last_modified"`
	Name         string    `json:"name"`

	// Keep Lock info
	Locked   bool     `json:"locked"`
	LockInfo LockInfo `json:"lock"`

	/*
	 * All fields below are copied from Terraform's code
	 * for compatibility
	 */

	// Version is the state file protocol version.
	Version int `json:"version"`

	// TFVersion is the version of Terraform that wrote this state.
	TFVersion string `json:"terraform_version,omitempty"`

	// Serial is incremented on any operation that modifies
	// the State file. It is used to detect potentially conflicting
	// updates.
	Serial int64 `json:"serial"`

	// Lineage is set when a new, blank state is created and then
	// never updated. This allows us to determine whether the serials
	// of two states can be meaningfully compared.
	// Apart from the guarantee that collisions between two lineages
	// are very unlikely, this value is opaque and external callers
	// should only compare lineage strings byte-for-byte for equality.
	Lineage string `json:"lineage"`

	// Remote is used to track the metadata required to
	// pull and push state files from a remote storage endpoint.
	Remote *terraform.RemoteState `json:"remote,omitempty"`

	// Backend tracks the configuration for the backend in use with
	// this state. This is used to track any changes in the backend
	// configuration.
	Backend *terraform.BackendState `json:"backend,omitempty"`

	// Modules contains all the modules in a breadth-first order
	Modules []*terraform.ModuleState `json:"modules"`
}

// Metadata is a metadata struct
type Metadata struct {
	Total int `json:"total"`
	Page  int `json:"page"`
}

// StateCollection is a collection of State, with metadata
type StateCollection struct {
	Metadata []*Metadata `json:"metadata"`
	Data     []*State    `json:"data"`
}

// LockInfo stores lock metadata.
//
// Copied from Terraform's source code
// to add missing json tags to structure fields
type LockInfo struct {
	// Unique ID for the lock. NewLockInfo provides a random ID, but this may
	// be overridden by the lock implementation. The final value if ID will be
	// returned by the call to Lock.
	ID string `json:"id,omitempty"`

	// Terraform operation, provided by the caller.
	Operation string `json:"operation,omitempty"`

	// Extra information to store with the lock, provided by the caller.
	Info string `json:"info,omitempty"`

	// user@hostname when available
	Who string `json:"who,omitempty"`

	// Terraform version
	Version string `json:"version,omitempty"`

	// Time that the lock was taken.
	// Use a *time.Time to allow omitempty
	Created *time.Time `json:"created,omitempty"`

	// Path to the state file when applicable. Set by the Lock implementation.
	Path string `json:"path,omitempty"`
}

// ErrNoDocuments
var ErrNoDocuments = errors.New("No document found")

// Storage is an abstraction over database engines
type Storage interface {
	GetName() string
	ListStates(pageNum, pageSize int) (coll StateCollection, err error)
	GetState(name string, serial int) (state State, err error)
	InsertState(document State, timestamp, source, name string) (err error)
	RemoveState(name string) (err error)
	GetLockStatus(name string) (lockStatus LockInfo, err error)
	LockState(name string, lockData LockInfo) (err error)
	UnlockState(name string, lockData LockInfo) (err error)
	ListStateSerials(name string, pageNum, pageSize int) (coll StateCollection, err error)
}
