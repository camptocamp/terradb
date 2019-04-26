package storage

import (
	"errors"
	"time"

	"github.com/hashicorp/terraform/terraform"
)

// Document associates a name and a state
type Document struct {
	Timestamp    string           `json:"-"`
	LastModified time.Time        `json:"last_modified"`
	Name         string           `json:"name"`
	State        *terraform.State `json:"state"`
}

// ErrNoDocuments
var ErrNoDocuments = errors.New("No document found")

// Storage is an abstraction over database engines
type Storage interface {
	GetName() string
	ListStates(page_num, page_size int) (states []Document, err error)
	GetState(name string, serial int) (doc Document, err error)
	InsertState(document terraform.State, timestamp, source, name string) (err error)
	RemoveState(name string) (err error)
	GetLockStatus(name string) (lockStatus interface{}, err error)
	LockState(name string, lockData interface{}) (err error)
	UnlockState(name string, lockData interface{}) (err error)
}
