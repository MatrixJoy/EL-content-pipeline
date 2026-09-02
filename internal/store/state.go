package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var bucket = []byte("urls")

type State struct {
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}
type StateDB struct{ db *bolt.DB }

func OpenState(path string) (*StateDB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if err = db.Update(func(tx *bolt.Tx) error { _, e := tx.CreateBucketIfNotExists(bucket); return e }); err != nil {
		db.Close()
		return nil, err
	}
	return &StateDB{db: db}, nil
}
func (s *StateDB) Close() error { return s.db.Close() }
func (s *StateDB) Done(url string) bool {
	var done bool
	_ = s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucket).Get([]byte(url))
		var x State
		if json.Unmarshal(v, &x) == nil {
			done = x.Status == "complete" || x.Status == "skipped"
		}
		return nil
	})
	return done
}
func (s *StateDB) Set(url, status, message string) error {
	x, _ := json.Marshal(State{Status: status, Error: message, UpdatedAt: time.Now().UTC()})
	return s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucket).Put([]byte(url), x) })
}
