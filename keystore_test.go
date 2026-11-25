package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenPassword(t *testing.T) {
	a, b := genPassword(), genPassword()
	if len(a) != 16 || len(b) != 16 {
		t.Fatalf("password length: got %d and %d, want 16", len(a), len(b))
	}
	for _, c := range a + b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			t.Fatalf("password has a non-alphanumeric character: %q", a)
		}
	}
	if a == b {
		t.Fatal("two generated passwords are identical")
	}
}

func TestPropsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local.properties")
	// A file with a comment, an sdk.dir and an old signing key.
	if err := os.WriteFile(path, []byte("# comment\nsdk.dir=/opt/android-sdk\nreleaseStoreFile=/old/keystore.jks\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	updatePropsFile(path, map[string]string{
		"releaseStoreFile":     "/new/keystore.jks",
		"releaseStorePassword": "secret1",
		"releaseKeyAlias":      "release",
		"releaseKeyPassword":   "secret2",
	})
	got := readPropsFile(path)
	if got["sdk.dir"] != "/opt/android-sdk" {
		t.Fatalf("sdk.dir lost: %v", got)
	}
	if got["releaseStoreFile"] != "/new/keystore.jks" {
		t.Fatalf("releaseStoreFile not replaced: %v", got)
	}
	if got["releaseStorePassword"] != "secret1" || got["releaseKeyAlias"] != "release" || got["releaseKeyPassword"] != "secret2" {
		t.Fatalf("new keys missing: %v", got)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "# comment") {
		t.Fatal("comment line was lost by updatePropsFile")
	}
	// A missing file reads as empty and is created on write.
	missing := filepath.Join(dir, "key.properties")
	if got := readPropsFile(missing); len(got) != 0 {
		t.Fatalf("missing file read as %v", got)
	}
	updatePropsFile(missing, map[string]string{"storeFile": "/x/y.jks"})
	if got := readPropsFile(missing); got["storeFile"] != "/x/y.jks" {
		t.Fatalf("new file not written: %v", got)
	}
}

func TestEnsureReleaseKeystore(t *testing.T) {
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// keytool from the real Temurin install, so no Java download happens.
	javaHome := filepath.Join(realHome, ".angrboda", "jdk", "temurin-17")
	if _, err := os.Stat(filepath.Join(javaHome, "bin", "keytool")); err != nil {
		t.Skip("no Temurin keytool at " + javaHome)
	}
	t.Setenv("JAVA_HOME", javaHome)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Kotlin project: no local.properties yet.
	proj := t.TempDir()
	ensureReleaseKeystore(proj)
	props := readPropsFile(filepath.Join(proj, "local.properties"))
	for _, k := range []string{"releaseStoreFile", "releaseStorePassword", "releaseKeyAlias", "releaseKeyPassword"} {
		if props[k] == "" {
			t.Fatalf("local.properties is missing %s: %v", k, props)
		}
	}
	if props["releaseKeyAlias"] != "release" {
		t.Fatalf("alias = %q, want release", props["releaseKeyAlias"])
	}
	ksPath := filepath.Join(tmpHome, ".angrboda", "keystore", "release.keystore")
	if _, err := os.Stat(ksPath); err != nil {
		t.Fatalf("keystore was not created at %s: %v", ksPath, err)
	}
	if props["releaseStoreFile"] != ksPath {
		t.Fatalf("releaseStoreFile = %q, want %q", props["releaseStoreFile"], ksPath)
	}
	shared := readPropsFile(filepath.Join(tmpHome, ".angrboda", "keystore", "release.properties"))
	if shared["storePassword"] == "" || shared["keyPassword"] == "" {
		t.Fatalf("release.properties is missing passwords: %v", shared)
	}
	// Second call is a no-op: the same keystore, nothing rewritten.
	before, _ := os.ReadFile(filepath.Join(proj, "local.properties"))
	ensureReleaseKeystore(proj)
	after, _ := os.ReadFile(filepath.Join(proj, "local.properties"))
	if string(before) != string(after) {
		t.Fatal("second ensureReleaseKeystore rewrote local.properties")
	}

	// Flutter project: android/key.properties instead.
	flutterProj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(flutterProj, "android"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flutterProj, "pubspec.yaml"), []byte("name: demo\nversion: 1.0.0+1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ensureReleaseKeystore(flutterProj)
	fprops := readPropsFile(filepath.Join(flutterProj, "android", "key.properties"))
	for _, k := range []string{"storeFile", "storePassword", "keyAlias", "keyPassword"} {
		if fprops[k] == "" {
			t.Fatalf("key.properties is missing %s: %v", k, fprops)
		}
	}
	if fprops["storeFile"] != ksPath {
		t.Fatalf("Flutter storeFile = %q, want the shared keystore %q", fprops["storeFile"], ksPath)
	}
}

func TestTemplateSigningBlocks(t *testing.T) {
	compose, err := composeTemplate.ReadFile("templates/compose/app/build.gradle.kts")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"signingConfigs", "releaseStoreFile", "hasReleaseKey"} {
		if !strings.Contains(string(compose), want) {
			t.Fatalf("compose template is missing %q", want)
		}
	}
	empty, err := emptyTemplate.ReadFile("templates/empty/app/build.gradle.kts")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"signingConfigs", "releaseStoreFile"} {
		if !strings.Contains(string(empty), want) {
			t.Fatalf("empty template is missing %q", want)
		}
	}
	if strings.Contains(string(empty), "compose-bom") || strings.Contains(string(empty), "material3") {
		t.Fatal("empty template must not mention Compose")
	}
	flutter, err := flutterTemplate.ReadFile("templates/flutter/android/app/build.gradle.kts")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"key.properties", "hasReleaseKey", "signingConfigs.getByName(\"debug\")"} {
		if !strings.Contains(string(flutter), want) {
			t.Fatalf("flutter template is missing %q", want)
		}
	}
	lib, err := libraryTemplate.ReadFile("templates/library/library/build.gradle.kts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(lib), "com.android.library") {
		t.Fatal("library template is missing com.android.library")
	}
	if strings.Contains(string(lib), "com.android.application") {
		t.Fatal("library template must not apply com.android.application")
	}
}
