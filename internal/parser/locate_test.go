package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeSave(t *testing.T, root, name string, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, name)
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(f, mtime, mtime)
	return f
}

func TestFindSavePicksMostRecent(t *testing.T) {
	root := t.TempDir()
	makeSave(t, root, "Old_1", time.Now().Add(-48*time.Hour))
	want := makeSave(t, root, "New_2", time.Now())
	got, err := FindSave(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestFindSaveByName(t *testing.T) {
	root := t.TempDir()
	want := makeSave(t, root, "Old_1", time.Now().Add(-48*time.Hour))
	makeSave(t, root, "New_2", time.Now())
	got, err := FindSave(root, "Old_1")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestFindSaveEmpty(t *testing.T) {
	if _, err := FindSave(t.TempDir(), ""); err == nil {
		t.Error("expected error for empty root")
	}
}

// A save folder is only valid when it contains a file named after itself;
// SaveGameInfo-only or stray folders must be ignored.
func TestFindSaveIgnoresFoldersWithoutSaveFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Bogus_9"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Bogus_9", "SaveGameInfo"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := makeSave(t, root, "Real_1", time.Now().Add(-time.Hour))
	got, err := FindSave(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestCandidateRootsAllExist(t *testing.T) {
	for _, root := range CandidateRoots() {
		fi, err := os.Stat(root)
		if err != nil || !fi.IsDir() {
			t.Errorf("CandidateRoots returned a non-directory: %s", root)
		}
	}
}
