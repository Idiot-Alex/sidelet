package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeProfileTransactionsAndReopen(t *testing.T) {
	dir := t.TempDir()
	p, err := OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	before := []byte(`{"schema":2,"tasks":[{"title":"旧标题","version":1}],"theme":"mac"}`)
	if err = p.Write("", before); err != nil {
		t.Fatal(err)
	}
	committed, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	// Fail after metadata is updated, proving the entire transaction rolls back.
	if _, err = p.db.Exec(`CREATE TRIGGER fail_task BEFORE UPDATE ON native_tasks BEGIN SELECT RAISE(ABORT, 'disk failure fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if err = p.Write("", []byte(`{"schema":2,"tasks":[{"title":"新标题","version":2}],"theme":"graphite"}`)); err == nil {
		t.Fatal("write unexpectedly succeeded")
	}
	after, err := p.Read()
	if err != nil || !bytes.Equal(committed, after) {
		t.Fatal("partial transaction committed", err)
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	p, err = OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	after, err = p.Read()
	if err != nil || !bytes.Equal(committed, after) {
		t.Fatal("reopen lost state", err)
	}
}

func TestNativeProfileRejectsUnknownAndFutureDatabases(t *testing.T) {
	for _, kind := range []string{"foreign", "future", "corrupt"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			p, err := OpenNativeProfile(dir)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "foreign":
				_, err = p.db.Exec("PRAGMA application_id=123")
			case "future":
				_, err = p.db.Exec("PRAGMA user_version=99")
			}
			if err != nil {
				t.Fatal(err)
			}
			p.Close()
			path := filepath.Join(dir, NativeProfileFilename)
			if kind == "corrupt" {
				if err = os.WriteFile(path, []byte("corrupt database"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(path)
			if db, e := OpenNativeProfile(dir); e == nil {
				db.Close()
				t.Fatal("invalid database accepted")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("invalid database modified")
			}
		})
	}
}
