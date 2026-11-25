package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveLocalTag(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "init")
	run("tag", "v1.0.1")

	// The tag exists now; removeLocalTag deletes it.
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "v1.0.1").Output(); err != nil {
		t.Fatalf("tag was not created: %v", err)
	} else if strings.TrimSpace(string(out)) == "" {
		t.Fatal("tag has no hash")
	}
	// removeLocalTag runs in the caller's directory, so chdir there.
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	removeLocalTag("v1.0.1")
	if err := exec.Command("git", "rev-parse", "v1.0.1").Run(); err == nil {
		t.Fatal("tag still exists after removeLocalTag")
	}
	// Removing a tag that is not there is a no-op, not a crash.
	removeLocalTag("v9.9.9")
}

func TestCertIsDebug(t *testing.T) {
	if !certIsDebug("CN=Android Debug, O=Android, C=US") {
		t.Fatal("debug DN not detected")
	}
	if certIsDebug("CN=Android Developer, OU=Development, O=Developer, C=US") {
		t.Fatal("release DN misread as debug")
	}
	if certIsDebug("") {
		t.Fatal("empty DN misread as debug")
	}
}

func TestVerifyApkSignedSkipsWithoutApksigner(t *testing.T) {
	// No ANDROID_HOME: findApksigner returns "" and the check warns
	// and passes, like the aapt stamp check does.
	t.Setenv("ANDROID_HOME", "")
	if err := verifyApkSigned("/nonexistent/app-release.apk"); err != nil {
		t.Fatalf("verifyApkSigned without apksigner should skip, got: %v", err)
	}
}

func TestVerifyApkSignedUnsigned(t *testing.T) {
	// apksigner is in the manually assembled SDK on this VM.
	home, _ := os.UserHomeDir()
	sdk := filepath.Join(home, "Android", "Sdk")
	apksigner := filepath.Join(sdk, "build-tools", "36.0.0", "apksigner")
	if _, err := os.Stat(apksigner); err != nil {
		t.Skip("no apksigner at " + apksigner)
	}
	t.Setenv("ANDROID_HOME", sdk)
	// A zip is not a signed APK: apksigner verify must fail.
	dir := t.TempDir()
	fake := filepath.Join(dir, "app-release.apk")
	if err := os.WriteFile(fake, []byte("PK\x03\x04not an apk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyApkSigned(fake); err == nil {
		t.Fatal("verifyApkSigned passed a file that is not a signed APK")
	} else if !strings.Contains(err.Error(), "NOT signed") {
		t.Fatalf("wrong error for unsigned APK: %v", err)
	}
}
