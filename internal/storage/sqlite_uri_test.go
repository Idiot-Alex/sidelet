package storage

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteFileURIDriveIsAPathNotAnAuthority(t *testing.T) {
	for _, tt := range []struct{ path, wantPath string }{
		{"C:/Users/Sidelet/sidelet.sqlite3", "/C:/Users/Sidelet/sidelet.sqlite3"},
		{"d:/资料 #1/任务?.sqlite3", "/d:/资料 #1/任务?.sqlite3"},
		{"/Users/Sidelet/资料 #1.sqlite3", "/Users/Sidelet/资料 #1.sqlite3"},
		{"/tmp/C:/literal.sqlite3", "/tmp/C:/literal.sqlite3"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			uri := sqliteFileURI(tt.path)
			uri.RawQuery = "mode=ro&immutable=1"
			got, err := url.Parse(uri.String())
			if err != nil || got.Host != "" || got.Path != tt.wantPath || got.Fragment != "" || got.Query().Get("mode") != "ro" {
				t.Fatalf("unsafe SQLite URI: %s (%+v, %v)", uri.String(), got, err)
			}
			if strings.Contains(tt.path, "#") && !strings.Contains(uri.String(), "%23") {
				t.Fatal("filename became a URI fragment")
			}
			if strings.Contains(tt.path, "?") && !strings.Contains(uri.String(), "%3F") {
				t.Fatal("filename became a URI query")
			}
		})
	}
}

func TestNativeProfileInUnicodeAndHashDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "资料 #1")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p, err := OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"profileId":"synthetic","tasks":[{"title":"中文任务"}]}`)
	if err := p.Write("", payload); err != nil {
		p.Close()
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	p, err = OpenNativeProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	data, err := p.Read()
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		ProfileID string `json:"profileId"`
		Tasks     []struct {
			Title string `json:"title"`
		} `json:"tasks"`
	}
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.ProfileID != "synthetic" || len(state.Tasks) != 1 || state.Tasks[0].Title != "中文任务" {
		t.Fatal("file URI did not preserve task data", state)
	}
}
