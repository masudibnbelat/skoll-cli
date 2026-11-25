package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A test server that serves content and honours Range requests,
// like a real file host (GitHub releases, dl.google.com) does.
func rangeServer(t *testing.T, content []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rng := r.Header.Get("Range"); rng != "" {
			var start int64
			if _, err := fmt.Sscanf(rng, "bytes=%d-", &start); err == nil && start > 0 && start < int64(len(content)) {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(content)-1, len(content)))
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)-int(start)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(content[start:])
				return
			}
			// Bad range: serve everything (a real server returns 416;
			// the client must cope with a 200 either way).
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		_, _ = w.Write(content)
	}))
}

func TestDownloadResumes(t *testing.T) {
	content := []byte(strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789", 20000)) // ~720 KB
	srv := rangeServer(t, content)
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "sdk.zip")
	tmp := dest + ".tmp"
	// A half-downloaded file, as if the previous run was killed.
	half := len(content) / 2
	if err := os.WriteFile(tmp, content[:half], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := downloadFileProgress(srv.URL, dest, "test file"); err != nil {
		t.Fatalf("resumed download failed: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("resumed file is wrong: got %d bytes, want %d", len(got), len(content))
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("tmp file was not renamed away after the download")
	}
	// A second call skips the download entirely (file exists).
	if err := downloadFileProgress(srv.URL, dest, "test file"); err != nil {
		t.Fatalf("second download should be a no-op: %v", err)
	}
}

func TestDownloadRestartsWhenNoRange(t *testing.T) {
	// A server that ignores Range and always serves 200: the client
	// must start over, not append the new bytes to the old half.
	content := []byte(strings.Repeat("x", 100000))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "sdk.zip")
	if err := os.WriteFile(dest+".tmp", []byte(strings.Repeat("y", 50000)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := downloadFileProgress(srv.URL, dest, "test file"); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("file is wrong after a no-Range restart: got %d bytes, want %d", len(got), len(content))
	}
}

func TestCheckDiskSpace(t *testing.T) {
	dir := t.TempDir()
	free, ok := freeDiskSpace(dir)
	if !ok {
		t.Skip("free disk space could not be read here")
	}
	if free == 0 {
		t.Fatal("free disk space reported as 0")
	}
	// A small need passes; an absurd need fails with a clear message.
	if err := checkDiskSpace(filepath.Join(dir, "f.zip"), 1024); err != nil {
		t.Fatalf("small need should pass: %v", err)
	}
	if err := checkDiskSpace(filepath.Join(dir, "f.zip"), int64(free)+10*1024*1024*1024); err == nil {
		t.Fatal("absurd need should fail")
	} else if !strings.Contains(err.Error(), "not enough disk space") {
		t.Fatalf("wrong error for a full disk: %v", err)
	}
	// Unknown size never fails.
	if err := checkDiskSpace(filepath.Join(dir, "f.zip"), -1); err != nil {
		t.Fatalf("unknown size should pass: %v", err)
	}
}

func TestCopyBuiltBundle(t *testing.T) {
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "app", "build", "outputs", "bundle", "release")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "app-release.aab"), []byte("fake aab"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest, err := copyBuiltBundle(dir, "release")
	if err != nil {
		t.Fatalf("copyBuiltBundle failed: %v", err)
	}
	if dest != filepath.Join(dir, "bundle", filepath.Base(dir)+"-release.aab") {
		t.Fatalf("bundle copied to %q", dest)
	}
	if got, err := os.ReadFile(dest); err != nil || string(got) != "fake aab" {
		t.Fatalf("bundle copy is wrong: %v %q", err, got)
	}
	// No .aab at all: an error, not a die().
	if _, err := copyBuiltBundle(t.TempDir(), "release"); err == nil {
		t.Fatal("copyBuiltBundle should fail with no .aab")
	}
}

func TestKeystoreExportImport(t *testing.T) {
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	javaHome := filepath.Join(realHome, ".angrboda", "jdk", "temurin-17")
	if _, err := os.Stat(filepath.Join(javaHome, "bin", "keytool")); err != nil {
		t.Skip("no Temurin keytool at " + javaHome)
	}
	t.Setenv("JAVA_HOME", javaHome)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a keystore on this "machine".
	ensureSharedKeystore()
	orig, err := os.ReadFile(releaseKeystorePath())
	if err != nil {
		t.Fatal(err)
	}
	origProps, err := os.ReadFile(releaseKeystorePropsPath())
	if err != nil {
		t.Fatal(err)
	}

	// Export it.
	exportDir := filepath.Join(t.TempDir(), "backup")
	cmdKeystoreExport(exportDir)
	for _, name := range []string{"release.keystore", "release.properties"} {
		if _, err := os.Stat(filepath.Join(exportDir, name)); err != nil {
			t.Fatalf("export is missing %s: %v", name, err)
		}
	}

	// "New machine": wipe the keystore dir, then import.
	if err := os.RemoveAll(keystoreDir()); err != nil {
		t.Fatal(err)
	}
	cmdKeystoreImport(exportDir)
	got, err := os.ReadFile(releaseKeystorePath())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(orig) {
		t.Fatal("imported keystore differs from the exported one")
	}
	gotProps, err := os.ReadFile(releaseKeystorePropsPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(gotProps) != string(origProps) {
		t.Fatal("imported release.properties differs from the exported one")
	}
	// Importing again refuses (a keystore already exists here).
	// cmdKeystoreImport calls die() -> os.Exit(1), so this is only
	// checked by hand: the guard is the os.Stat above.
	if _, err := os.Stat(releaseKeystorePath()); err != nil {
		t.Fatal("keystore should exist after import")
	}
}
