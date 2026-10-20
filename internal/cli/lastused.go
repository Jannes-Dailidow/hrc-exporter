package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// Cache file names for values remembered from the last successful login.
const (
	lastUserFile    = "last-user"
	lastBaseURLFile = "last-base-url"
)

// lastUsed remembers a value of the most recent successful login (e.g. the
// personnel number) in the user cache dir, to prefill the prompt next time.
type lastUsed struct{ path string }

func newLastUsed(name string) lastUsed {
	dir, err := os.UserCacheDir()
	if err != nil {
		return lastUsed{}
	}
	return lastUsed{path: filepath.Join(dir, appName, name)}
}

func (l lastUsed) load() string {
	if l.path == "" {
		return ""
	}
	data, err := os.ReadFile(l.path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (l lastUsed) save(value string) error {
	if l.path == "" || value == "" || value == l.load() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(l.path, []byte(value+"\n"), 0o600)
}
