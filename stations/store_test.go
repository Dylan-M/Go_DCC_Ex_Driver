package stations

import (
	"errors"
	bolt "go.etcd.io/bbolt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStationPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "stations.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := Profile{Name: " Layout ", Mode: "TCP", Host: " localhost ", Port: 2560}
	if err := s.Save(p, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(Profile{Name: "USB", Mode: "Serial", Device: "COM4", Baud: 115200}, false); err != nil {
		t.Fatal(err)
	}
	duplicate := Profile{Name: "layout", Mode: "TCP", Host: "other", Port: 1234}
	if err := s.Save(duplicate, false); !errors.Is(err, ErrExists) {
		t.Fatal("duplicate", err)
	}
	got, err := s.List()
	if err != nil || len(got) != 2 || got[0].Host != "localhost" {
		t.Fatal(got, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	reopened, err := s.List()
	if err != nil || !reflect.DeepEqual(got, reopened) {
		t.Fatal("persistence", reopened, err)
	}
	if err := s.Save(duplicate, true); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.List()
	if updated[0].Host != "other" {
		t.Fatal("replace not applied")
	}
	if err := s.Delete("LAYOUT"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("layout"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	left, _ := s.List()
	if len(left) != 1 || left[0].Device != "COM4" {
		t.Fatal(left)
	}
}

func TestValidationAndLockErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stations.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, p := range []Profile{
		{}, {Name: "x", Mode: "TCP", Host: "host", Port: 0}, {Name: "x", Mode: "TCP", Host: "https://host", Port: 2560},
		{Name: "x", Mode: "TCP", Host: "localhost:2560", Port: 2560}, {Name: "x", Mode: "Serial", Device: "COM1", Baud: 0},
		{Name: "x\n", Mode: "unsupported"}, {Name: "a\x00b", Mode: "TCP", Host: "host", Port: 2560},
	} {
		if s.Save(p, false) == nil {
			t.Fatal("accepted", p)
		}
	}
	if got, _ := s.List(); len(got) != 0 {
		t.Fatal("invalid record persisted")
	}
	locked, err := Open(path)
	if err == nil {
		locked.Close()
		t.Fatal("second writer accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); err == nil {
		t.Fatal("closed DB read accepted")
	}
}

func TestFutureSchemaNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stations.db")
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket(metadata)
		if err != nil {
			return err
		}
		return b.Put([]byte("schema"), []byte("999"))
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("future schema accepted")
	}
	db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.View(func(tx *bolt.Tx) error {
		if string(tx.Bucket(metadata).Get([]byte("schema"))) != "999" || tx.Bucket(records) != nil {
			t.Fatal("future schema mutated")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeHost(t *testing.T) {
	for in, want := range map[string]string{"localhost": "localhost", " 192.168.4.1 ": "192.168.4.1", "[::1]": "::1", "::1": "::1", "fe80::1%eth0": "fe80::1%eth0"} {
		got, err := NormalizeHost(in)
		if err != nil || got != want {
			t.Fatal(in, got, err)
		}
	}
}
