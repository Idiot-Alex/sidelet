package storage

import (
	"net/url"
	"path/filepath"
)

// sqliteFileURI takes an absolute filesystem path. A Windows drive must be in
// the URI path (/C:/...), not its authority (file://C:/...), which SQLite rejects.
// URL handles escaping spaces, Unicode, '#' and '?' without changing query flags.
func sqliteFileURI(path string) url.URL {
	path = filepath.ToSlash(path)
	if len(path) >= 3 && path[1] == ':' && path[2] == '/' &&
		(path[0] >= 'A' && path[0] <= 'Z' || path[0] >= 'a' && path[0] <= 'z') {
		path = "/" + path
	}
	return url.URL{Scheme: "file", Path: path}
}
