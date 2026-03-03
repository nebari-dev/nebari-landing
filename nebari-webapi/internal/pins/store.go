// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package pins provides a bbolt-backed persistent store for per-user
// pinned-service preferences in the service-discovery API.
//
// Data model:
//
//	bucket "pins"
//	  key:   preferred_username  (string, from validated JWT claim)
//	  value: JSON-encoded []string of service UIDs
package pins

import (
	"encoding/json"
	"fmt"

	bbolt "go.etcd.io/bbolt"
)

const bucketName = "pins"

// PinStore persists per-user pinned service UIDs using bbolt.
type PinStore struct {
	db *bbolt.DB
}

// NewPinStore opens (or creates) the bbolt database at dbPath and returns a
// ready-to-use PinStore. The caller is responsible for calling Close() when done.
func NewPinStore(dbPath string) (*PinStore, error) {
	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening pin store at %q: %w", dbPath, err)
	}
	// Ensure the bucket exists.
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating pins bucket: %w", err)
	}
	return &PinStore{db: db}, nil
}

// Close releases the underlying database file.
func (s *PinStore) Close() error {
	return s.db.Close()
}

// Get returns the list of pinned UIDs for username.
// Returns an empty slice when the user has no pins stored yet.
func (s *PinStore) Get(username string) ([]string, error) {
	var uids []string
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(bucketName)).Get([]byte(username))
		if v == nil {
			uids = []string{}
			return nil
		}
		return json.Unmarshal(v, &uids)
	})
	if err != nil {
		return nil, err
	}
	return uids, nil
}

// Pin adds uid to username's pinned set (idempotent).
func (s *PinStore) Pin(username, uid string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		uids, err := decode(b.Get([]byte(username)))
		if err != nil {
			return err
		}
		for _, existing := range uids {
			if existing == uid {
				return nil // already pinned
			}
		}
		uids = append(uids, uid)
		return put(b, username, uids)
	})
}

// Unpin removes uid from username's pinned set (idempotent).
func (s *PinStore) Unpin(username, uid string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		uids, err := decode(b.Get([]byte(username)))
		if err != nil {
			return err
		}
		result := uids[:0]
		for _, existing := range uids {
			if existing != uid {
				result = append(result, existing)
			}
		}
		return put(b, username, result)
	})
}

// decode unmarshals a JSON-encoded []string from raw; returns [] on nil input.
func decode(raw []byte) ([]string, error) {
	if raw == nil {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// put JSON-encodes uids and stores them under key username.
func put(b *bbolt.Bucket, username string, uids []string) error {
	v, err := json.Marshal(uids)
	if err != nil {
		return err
	}
	return b.Put([]byte(username), v)
}
