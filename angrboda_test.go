package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveLatestFlutter(t *testing.T) {
	ver, archive := resolveLatestFlutter()
	t.Logf("flutter: %s %s", ver, archive)
	if ver == "" || archive == "" {
		t.Fatalf("empty version/archive")
	}
	if archive != "flutter_linux_"+ver+"-stable.tar.xz" {
		t.Fatalf("unexpected archive name: %s", archive)
	}
}

func TestLatestReleaseTag(t *testing.T) {
	tag, err := latestReleaseTag()
	if err != nil {
		t.Fatalf("latestReleaseTag: %v", err)
	}
	t.Logf("latest tag: %s", tag)
	if tag == "" || tag[0] != 'v' {
		t.Fatalf("bad tag: %q", tag)
	}
}

func TestSdkApiLevel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "build.gradle.kts"), []byte("android {\n  compileSdk = 35\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := sdkApiLevel(root); got != 35 {
		t.Fatalf("sdkApiLevel = %d, want 35", got)
	}
	if got := sdkApiLevel(t.TempDir()); got != 36 {
		t.Fatalf("sdkApiLevel (missing) = %d, want 36", got)
	}
}

func TestFindAndroidSdk(t *testing.T) {
	root := t.TempDir()
	sdk := filepath.Join(root, "Sdk")
	if err := os.MkdirAll(filepath.Join(sdk, "cmdline-tools", "latest", "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdk, "cmdline-tools", "latest", "bin", "sdkmanager"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDROID_HOME", sdk)
	t.Setenv("ANDROID_SDK_ROOT", "")
	if got := findAndroidSdk(); got != sdk {
		t.Fatalf("findAndroidSdk = %q, want %q", got, sdk)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:                 "0 B",
		512:               "512 B",
		1536:              "1.5 KB",
		9 * 1024 * 1024:   "9 MB",
		690 * 1024 * 1024: "690 MB",
		1518418752:        "1.4 GB",
		114700:            "112 KB",
		9571024:           "9.1 MB",
	}
	for n, want := range cases {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		45 * time.Second:        "45s",
		65 * time.Second:        "1m 05s",
		123 * time.Second:       "2m 03s",
		3720 * time.Second:      "1h 02m",
		1500 * time.Millisecond: "2s",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestProgressLine(t *testing.T) {
	// 690 MB of 1,518,418,752 bytes (47%) at 12.4 MB/s with 65s left.
	line := progressLine(690*1024*1024, 1518418752, 12.4*1024*1024, 65*time.Second)
	for _, want := range []string{"47%", "690 MB", "1.4 GB", "12.4 MB/s", "ETA 1m 05s"} {
		if !strings.Contains(line, want) {
			t.Errorf("progressLine = %q, missing %q", line, want)
		}
	}
	// Speed unknown yet: speed and ETA show as …
	line = progressLine(0, 1024*1024, 0, 0)
	if !strings.Contains(line, "0%") || !strings.Contains(line, "…") {
		t.Errorf("progressLine (no speed) = %q", line)
	}
}

func TestUpdateAvailable(t *testing.T) {
	if !updateAvailable("v0.8.1", "0.8.0") {
		t.Error("v0.8.1 should be newer than 0.8.0")
	}
	if updateAvailable("v0.8.0", "0.8.0") {
		t.Error("same version should not be an update")
	}
	if updateAvailable("v0.7.0", "0.8.0") {
		t.Error("older version should not be an update")
	}
	if updateAvailable("", "0.8.0") {
		t.Error("empty latest should not be an update")
	}
}

func TestNeedsUpdateCheck(t *testing.T) {
	now := time.Now()
	if !needsUpdateCheck(updateCache{}, now) {
		t.Error("missing cache should need a check")
	}
	fresh := updateCache{Latest: "v0.8.0", CheckedAt: now.Add(-time.Hour)}
	if needsUpdateCheck(fresh, now) {
		t.Error("1-hour-old cache should not need a check")
	}
	stale := updateCache{Latest: "v0.8.0", CheckedAt: now.Add(-25 * time.Hour)}
	if !needsUpdateCheck(stale, now) {
		t.Error("25-hour-old cache should need a check")
	}
}

func TestUpdateCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update-check.json")
	when := time.Now().Truncate(time.Second)
	writeUpdateCache(path, updateCache{Latest: "v0.8.1", CheckedAt: when})
	got := readUpdateCache(path)
	if got.Latest != "v0.8.1" {
		t.Fatalf("cache latest = %q, want v0.8.1", got.Latest)
	}
	if !got.CheckedAt.Equal(when) {
		t.Fatalf("cache checked_at = %v, want %v", got.CheckedAt, when)
	}
	if got := readUpdateCache(filepath.Join(t.TempDir(), "missing.json")); got.Latest != "" {
		t.Fatalf("missing cache should be empty, got %+v", got)
	}
}
