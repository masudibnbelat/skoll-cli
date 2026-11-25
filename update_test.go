package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyExecutable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("hello angrboda"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copyExecutable(src, dst); err != nil {
		t.Fatalf("copyExecutable: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello angrboda" {
		t.Fatalf("dst = %q, want hello angrboda", got)
	}
	if st, err := os.Stat(dst); err != nil {
		t.Fatal(err)
	} else if st.Mode().Perm() != 0755 {
		t.Fatalf("dst mode = %o, want 0755", st.Mode().Perm())
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("src still exists after copyExecutable")
	}
}

func TestMoveFileSameDevice(t *testing.T) {
	// Same filesystem: plain rename path.
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := moveFile(src, dst); err != nil {
		t.Fatalf("moveFile: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("dst missing: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("src still exists after moveFile")
	}
}
