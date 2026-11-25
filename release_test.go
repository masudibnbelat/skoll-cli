package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBumpKotlinVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	gradle := `android {
    defaultConfig {
        applicationId = "com.example.demo"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"
    }
}
`
	path := filepath.Join(dir, "app", "build.gradle.kts")
	if err := os.WriteFile(path, []byte(gradle), 0o644); err != nil {
		t.Fatal(err)
	}
	bumpKotlinVersion(dir, "v1.0.1")
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), `versionName = "1.0.1"`) {
		t.Fatalf("versionName not bumped:\n%s", out)
	}
	if !strings.Contains(string(out), "versionCode = 2") {
		t.Fatalf("versionCode not bumped:\n%s", out)
	}
	if codeVersion(dir, "kotlin") != "1.0.1" {
		t.Fatalf("codeVersion = %q, want 1.0.1", codeVersion(dir, "kotlin"))
	}
}

func TestBumpPubspecVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pubspec.yaml")
	if err := os.WriteFile(path, []byte("name: demo\ndescription: demo app\nversion: 1.0.0+1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ver, build, ok := parsePubspecVersion(dir); !ok || ver != "1.0.0" || build != 1 {
		t.Fatalf("parsePubspecVersion = %q, %d, %v", ver, build, ok)
	}
	bumpPubspecVersion(dir, "v1.0.1")
	out, _ := os.ReadFile(path)
	if !strings.Contains(string(out), "version: 1.0.1+2") {
		t.Fatalf("pubspec not bumped:\n%s", out)
	}
	if codeVersion(dir, "flutter") != "1.0.1" {
		t.Fatalf("codeVersion = %q, want 1.0.1", codeVersion(dir, "flutter"))
	}
	// No build number yet: it starts at 1.
	if err := os.WriteFile(path, []byte("name: demo\nversion: 2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bumpPubspecVersion(dir, "v2.3.5")
	out, _ = os.ReadFile(path)
	if !strings.Contains(string(out), "version: 2.3.5+1") {
		t.Fatalf("pubspec without build number not bumped:\n%s", out)
	}
}

func TestParsePubspecVersionBad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pubspec.yaml")
	if err := os.WriteFile(path, []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := parsePubspecVersion(dir); ok {
		t.Fatal("parsePubspecVersion accepted a pubspec with no version")
	}
}

func TestIsLibraryProject(t *testing.T) {
	// An app project is not a library.
	app := t.TempDir()
	if err := os.MkdirAll(filepath.Join(app, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "app", "build.gradle.kts"), []byte("plugins {\n    id(\"com.android.application\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isLibraryProject(app) {
		t.Fatal("app project reported as library")
	}
	if projectKind(app) != "kotlin" {
		t.Fatalf("projectKind(app) = %q, want kotlin", projectKind(app))
	}
	// A library project is.
	lib := t.TempDir()
	if err := os.MkdirAll(filepath.Join(lib, "library"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "library", "build.gradle.kts"), []byte("plugins {\n    id(\"com.android.library\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isLibraryProject(lib) {
		t.Fatal("library project not detected")
	}
	if projectKind(lib) != "library" {
		t.Fatalf("projectKind(lib) = %q, want library", projectKind(lib))
	}
	// A Flutter project wins over both.
	fl := t.TempDir()
	if err := os.WriteFile(filepath.Join(fl, "pubspec.yaml"), []byte("name: demo\nversion: 1.0.0+1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if projectKind(fl) != "flutter" {
		t.Fatalf("projectKind(flutter) = %q, want flutter", projectKind(fl))
	}
}

func TestApplyVersionPins(t *testing.T) {
	// No pins: the looked-up versions pass through unchanged.
	newGradlePin, newAgpPin, newKotlinPin, newBomPin = "", "", "", ""
	v := applyVersionPins(pinnedVersions)
	if v.gradle != "9.3.1" || v.agp != "9.1.1" || v.kotlin != "2.2.10" || v.bom != "2025.09.00" {
		t.Fatalf("no-pin apply changed versions: %+v", v)
	}
	newGradlePin, newAgpPin, newKotlinPin, newBomPin = "9.3.1", "9.1.1", "2.2.10", "2025.09.00"
	defer func() { newGradlePin, newAgpPin, newKotlinPin, newBomPin = "", "", "", "" }()
	v = applyVersionPins(appVersions{gradle: "9.9.2", agp: "9.5.2", kotlin: "2.4.21", bom: "2026.11.01"})
	if v.gradle != "9.3.1" || v.agp != "9.1.1" || v.kotlin != "2.2.10" || v.bom != "2025.09.00" {
		t.Fatalf("pins not applied: %+v", v)
	}
	if !anyPinSet() {
		t.Fatal("anyPinSet is false with pins set")
	}
}
