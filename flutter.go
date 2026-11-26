package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Flutter SDK auto-install. A system Flutter wins; otherwise the
// latest stable SDK is downloaded once to ~/.skoll/flutter/ and
// added to PATH for the child processes. Nothing here needs Android
// Studio — but a Flutter build still needs the Android SDK, which
// ensureAndroidSdk installs (see sdk.go).

// flutterPinnedVersion is the fallback when the latest stable
// cannot be looked up (offline).
const flutterPinnedVersion = "3.50.1"

func flutterSdkDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".skoll", "flutter")
}

func flutterBin() string {
	name := "flutter"
	if runtime.GOOS == "windows" {
		name = "flutter.bat"
	}
	return filepath.Join(flutterSdkDir(), "bin", name)
}

// findFlutter returns (flutterBin, source): "system", "skoll", or "".
func findFlutter() (string, string) {
	if p, err := lookPath("flutter"); err == nil {
		return p, "system"
	}
	if p := flutterBin(); fileExists(p) {
		return p, "skoll"
	}
	return "", ""
}

// ensureFlutter makes sure `flutter` is available: a system Flutter
// wins, else the skoll cache, else the latest stable SDK is
// downloaded once (~800 MB) and added to PATH.
func ensureFlutter() {
	if p, _ := findFlutter(); p != "" {
		addToPath(filepath.Dir(p))
		return
	}
	ver, archive := resolveLatestFlutter()
	stepLine(fmt.Sprintf("Flutter SDK not found — downloading Flutter %s once (large — over 1 GB)…", ver))
	if err := installFlutterSdk(ver, archive); err != nil {
		die("could not install the Flutter SDK (" + err.Error() + "). Install Flutter once yourself (https://docs.flutter.dev/get-started/install) and retry.")
	}
	p, _ := findFlutter()
	if p == "" {
		die("Flutter SDK installed but bin/flutter was not found after extract.")
	}
	addToPath(filepath.Dir(p))
	okLine("Flutter ready: " + p)
	// First run: let the tool finish its own setup and prove it works.
	if err := runLive("flutter", "--version"); err != nil {
		die("flutter --version failed after install: " + err.Error())
	}
}

// resolveLatestFlutter returns (version, archive) for the latest
// stable Flutter SDK for this OS, from the official releases JSON.
// Offline or on error it returns the pinned version.
func resolveLatestFlutter() (string, string) {
	jsonName := map[string]string{
		"linux":   "releases_linux.json",
		"darwin":  "releases_macos.json",
		"windows": "releases_windows.json",
	}[runtime.GOOS]
	if jsonName == "" {
		return flutterPinnedVersion, flutterArchive(flutterPinnedVersion)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://storage.googleapis.com/flutter_infra_release/releases/" + jsonName)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var data struct {
			CurrentRelease struct {
				Stable string `json:"stable"`
			} `json:"current_release"`
			Releases []struct {
				Hash    string `json:"hash"`
				Version string `json:"version"`
				Archive string `json:"archive"`
			} `json:"releases"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err == nil {
			for _, r := range data.Releases {
				if r.Hash == data.CurrentRelease.Stable && r.Archive != "" {
					// The JSON archive is a path from the releases
					// root ("stable/linux/flutter_linux_...tar.xz");
					// installFlutterSdk adds the stable/<os>/ part,
					// so only the file name is returned.
					archive := r.Archive
					if i := strings.LastIndex(archive, "/"); i >= 0 {
						archive = archive[i+1:]
					}
					return r.Version, archive
				}
			}
		}
	}
	return flutterPinnedVersion, flutterArchive(flutterPinnedVersion)
}

// flutterArchive builds the archive name for a version on this OS
// (the layout of the official release bundles).
func flutterArchive(ver string) string {
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "flutter_linux_arm64_" + ver + "-stable.tar.xz"
		}
		return "flutter_linux_" + ver + "-stable.tar.xz"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "flutter_macos_arm64_" + ver + "-stable.zip"
		}
		return "flutter_macos_" + ver + "-stable.zip"
	case "windows":
		return "flutter_windows_" + ver + "-stable.zip"
	}
	return ""
}

func flutterReleaseOs() string {
	switch runtime.GOOS {
	case "linux":
		return "linux"
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	}
	return ""
}

func installFlutterSdk(ver, archive string) error {
	osName := flutterReleaseOs()
	if osName == "" {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	url := "https://storage.googleapis.com/flutter_infra_release/releases/stable/" + osName + "/" + archive
	home, _ := os.UserHomeDir()
	dlDir := filepath.Join(home, ".skoll", "downloads")
	if err := os.MkdirAll(dlDir, 0755); err != nil {
		return err
	}
	archivePath := filepath.Join(dlDir, archive)
	if err := downloadFileProgress(url, archivePath, "Flutter "+ver); err != nil {
		return err
	}
	dest := flutterSdkDir()
	_ = os.RemoveAll(dest)
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	if strings.HasSuffix(archive, ".tar.xz") {
		// Go has no xz support: the system tar does (Linux, macOS,
		// and Windows 10+ all ship tar).
		if err := runLive("tar", "-xf", archivePath, "-C", parent); err != nil {
			return fmt.Errorf("tar extract failed (%v) — install tar once yourself", err)
		}
	} else {
		if err := unzip(archivePath, parent); err != nil {
			return err
		}
	}
	// The bundle extracts to <parent>/flutter — that IS dest.
	if !fileExists(flutterBin()) {
		return fmt.Errorf("Flutter %s downloaded but bin/flutter was not found after extract", ver)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(flutterBin(), 0755)
	}
	return nil
}
