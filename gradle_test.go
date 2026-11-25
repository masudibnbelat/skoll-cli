package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGradleReadOnly(t *testing.T) {
	// A wrapper script wins, and nothing is downloaded or changed.
	dir := t.TempDir()
	wrapper := filepath.Join(dir, gradlewName())
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findGradleReadOnly(dir); got != wrapper {
		t.Fatalf("wrapper project: got %q, want %q", got, wrapper)
	}

	// A wrapper version with no script and no local install: "" —
	// and no download is attempted (the build downloads it later).
	// Use a version that cannot exist locally.
	dir2 := t.TempDir()
	propsDir := filepath.Join(dir2, "gradle", "wrapper")
	if err := os.MkdirAll(propsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	props := "distributionUrl=https\\://services.gradle.org/distributions/gradle-99.99.99-bin.zip\n"
	if err := os.WriteFile(filepath.Join(propsDir, "gradle-wrapper.properties"), []byte(props), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findGradleReadOnly(dir2); got != "" {
		t.Fatalf("missing Gradle: got %q, want empty", got)
	}

	// Nothing at all: "".
	if got := findGradleReadOnly(t.TempDir()); got != "" {
		t.Fatalf("empty project: got %q, want empty", got)
	}
}
