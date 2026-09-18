package stations

import (
	"bytes"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/config"
	bolt "go.etcd.io/bbolt"
)

// Preferences share the station database and its lifetime, but not its records.
// Values use the config codecs; no live command-station state is stored here.
var preferences = []byte("preferences")

func (s *Store) readPreference(key string, decode func([]byte) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(preferences)
		if bucket == nil {
			return nil
		}
		if data := bucket.Get([]byte(key)); data != nil {
			return decode(data)
		}
		return nil
	})
}

func (s *Store) writePreference(key string, data []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(preferences)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(key), data)
	})
}

func (s *Store) LoadThrottles() (config.ThrottleSettings, error) {
	settings := config.DefaultThrottles()
	err := s.readPreference("throttles", func(data []byte) error {
		var err error
		settings, err = config.DecodeThrottles(bytes.NewReader(data))
		return err
	})
	return settings, err
}

func (s *Store) SaveThrottles(settings config.ThrottleSettings) error {
	var data bytes.Buffer
	if err := config.EncodeThrottles(&data, settings); err != nil {
		return err
	}
	return s.writePreference("throttles", data.Bytes())
}

func (s *Store) LoadSettings() (config.Settings, error) {
	settings := config.Default()
	err := s.readPreference("function-modes", func(data []byte) error {
		var err error
		settings, err = config.Decode(bytes.NewReader(data))
		return err
	})
	return settings, err
}

func (s *Store) SaveSettings(settings config.Settings) error {
	var data bytes.Buffer
	if err := config.Encode(&data, settings); err != nil {
		return err
	}
	return s.writePreference("function-modes", data.Bytes())
}
