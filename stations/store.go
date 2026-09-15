package stations

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

var ErrExists = errors.New("a station with that name already exists")
var ErrNotFound = errors.New("saved station not found")
var records = []byte("stations")
var metadata = []byte("metadata")

type Store struct{ db *bolt.DB }

// The caller supplies its platform's app-private storage directory. No desktop
// home-directory assumption or Fyne dependency exists in this layer.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("station database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open station database: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		meta, err := tx.CreateBucketIfNotExists(metadata)
		if err != nil {
			return err
		}
		version := meta.Get([]byte("schema"))
		if version != nil && string(version) != "1" {
			return errors.New("unsupported station database schema; file left unchanged")
		}
		if version == nil {
			if tx.Bucket(records) != nil {
				return errors.New("station database is missing its schema")
			}
			if err := meta.Put([]byte("schema"), []byte("1")); err != nil {
				return err
			}
		}
		_, err = tx.CreateBucketIfNotExists(records)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) List() ([]Profile, error) {
	profiles := []Profile{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(records).ForEach(func(key, value []byte) error {
			var p Profile
			if err := json.Unmarshal(value, &p); err != nil {
				return fmt.Errorf("read station %q: %w", key, err)
			}
			p, err := Normalize(p)
			if err != nil {
				return fmt.Errorf("invalid saved station %q: %w", key, err)
			}
			if string(key) != strings.ToLower(p.Name) {
				return errors.New("saved station key/name mismatch")
			}
			profiles = append(profiles, p)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(profiles, func(i, j int) bool { return strings.ToLower(profiles[i].Name) < strings.ToLower(profiles[j].Name) })
	return profiles, nil
}

func (s *Store) Save(p Profile, replace bool) error {
	p, err := Normalize(p)
	if err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(records)
		key := []byte(strings.ToLower(p.Name))
		if !replace && bucket.Get(key) != nil {
			return ErrExists
		}
		return bucket.Put(key, data)
	})
}

func (s *Store) Delete(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(records)
		key := []byte(strings.ToLower(strings.TrimSpace(name)))
		if bucket.Get(key) == nil {
			return ErrNotFound
		}
		return bucket.Delete(key)
	})
}
