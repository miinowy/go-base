// Package kv provide k/v storage by LevelDB
package kv

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/miinowy/go-base"
)

var (
	// Default leveldb
	dbd  *Context
	once sync.Once

	// Map of leveldb instance
	mtx   sync.Mutex
	dbMap sync.Map

	// ErrNotFound indicates record was not found in db
	ErrNotFound = fmt.Errorf("kv: not found")
	// ErrForbidden indicates can't access db
	ErrForbidden = fmt.Errorf("kv: forbidden")
)

// Initialize default leveldb
func trigger() {
	once.Do(func() {
		dbd = NewContext("default")
		if dbd == nil {
			panic(ErrForbidden)
		}
	})
}

// Kver provide key/value operates
type Kver interface {
	Has(key []byte) (ok bool, err error)
	Get(key []byte) (value []byte, err error)
	Put(key []byte, value []byte) (err error)
	Delete(key []byte) (err error)
}

// Context of kv client
type Context struct {
	name string
	db   *leveldb.DB
}

// NewContext return a new Context of kv
//
// Context was initialized at path that named `kv.name.path` in config file
// NewContext will return nil if no client found by name `kv.name.path`
func NewContext(name string) *Context {
	if value, ok := dbMap.Load(name); ok == true {
		return value.(*Context)
	}

	mtx.Lock()
	defer mtx.Unlock()
	if value, ok := dbMap.Load(name); ok == true {
		return value.(*Context)
	}

	dbpath := base.GetPath(viper.GetString(fmt.Sprintf("kv.%s.path", name)))
	db, err := leveldb.OpenFile(dbpath, nil)
	if err != nil {
		return nil
	}
	c := &Context{name: name, db: db}
	dbMap.Store(name, c)
	return c
}

// Has is used to check the existence of a key
func Has(key []byte) (bool, error) {
	trigger()
	return dbd.Has(key)
}

// Has is used to check the existence of a key
func (c *Context) Has(key []byte) (bool, error) {
	return c.db.Has(key, nil)
}

// Get value of key
func Get(key []byte) ([]byte, error) {
	trigger()
	return dbd.Get(key)
}

// Get value of key
func (c *Context) Get(key []byte) ([]byte, error) {
	return c.db.Get(key, nil)
}

// Put key/value
func Put(key []byte, value []byte) error {
	trigger()
	return dbd.Put(key, value)
}

// Put key/value
func (c *Context) Put(key []byte, value []byte) error {
	return c.db.Put(key, value, nil)
}

// Delete key/value
func Delete(key []byte) error {
	trigger()
	return dbd.Delete(key)
}

// Delete key/value
func (c *Context) Delete(key []byte) error {
	return c.db.Delete(key, nil)
}

// List keys
func List(start []byte, limit []byte) (list [][]byte) {
	trigger()
	return dbd.List(start, limit)
}

// List keys
func (c *Context) List(start []byte, limit []byte) (list [][]byte) {
	iter := c.db.NewIterator(&util.Range{Start: start, Limit: limit}, nil)
	for iter.Next() {
		list = append(list, append([]byte{}, iter.Key()...))
	}
	return list
}

// Compact storage
func Compact() error {
	trigger()
	return dbd.Compact()
}

// Compact storage
func (c *Context) Compact() error {
	return c.db.CompactRange(util.Range{})
}
