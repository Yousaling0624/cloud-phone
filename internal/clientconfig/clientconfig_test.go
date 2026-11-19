package clientconfig_test

import (
	"os"
	"path/filepath"
	"testing"

	"cloud-phone/internal/clientconfig"
)

func TestSaveLoadPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "client.json")
	if _, err := clientconfig.Load(path); err != nil {
		t.Fatalf("missing file should not be an error: %v", err)
	}
	want := clientconfig.Config{ServerURL: "http://127.0.0.1:8787", Token: "secret"}
	if err := clientconfig.Save(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := clientconfig.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Errorf("load = %+v, want %+v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 600", mode)
	}
}
