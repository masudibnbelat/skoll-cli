package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
)

// Android SDK auto-install. An existing SDK is used as-is; otherwise
// cmdline-tools 23.0 is downloaded once and sdkmanager installs
// platform-tools + the platform + build-tools for the project's API
// level. Nothing here needs Android Studio.

// defaultSdkDir: ANDROID_HOME / ANDROID_SDK_ROOT, else the usual
// default (~/Android/Sdk, or %LOCALAPPDATA%\Android\Sdk on Windows).
func defaultSdkDir() string {
	if sdk := os.Getenv("ANDROID_HOME"); sdk != "" {
		return sdk
	}
	if sdk := os.Getenv("ANDROID_SDK_ROOT"); sdk != "" {
		return sdk
	}
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			return filepath.Join(la, "Android", "Sdk")
		}
		return filepath.Join(home, "AppData", "Local", "Android", "Sdk")
	}
	return filepath.Join(home, "Android", "Sdk")
}

func sdkmanagerPath(sdkDir string) string {
	name := "sdkmanager"
	if runtime.GOOS == "windows" {
		name = "sdkmanager.bat"
	}
	return filepath.Join(sdkDir, "cmdline-tools", "latest", "bin", name)
}

// findAndroidSdk returns the SDK dir when it looks installed
// (cmdline-tools present, or a platforms dir already there).
// "" when nothing usable exists.
func findAndroidSdk() string {
	dir := defaultSdkDir()
	if dir == "" {
		return ""
	}
	if fileExists(sdkmanagerPath(dir)) {
		return dir
	}
	// A hand-installed SDK without cmdline-tools still counts if the
	// platform dir exists.
	if st, err := os.Stat(filepath.Join(dir, "platforms")); err == nil && st.IsDir() {
		return dir
	}
	return ""
}

// sdkApiLevel reads the project's compileSdk: app/build.gradle.kts
// (compileSdk = 36). Flutter projects use flutter.compileSdkVersion,
// which is 36, so they fall back to 36 too.
func sdkApiLevel(root string) int {
	for _, name := range []string{
		filepath.Join("app", "build.gradle.kts"),
		filepath.Join("app", "build.gradle"),
	} {
		if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			if m := regexp.MustCompile(`compileSdk(?:Version)?\s*=?\s*(\d+)`).FindSubmatch(data); m != nil {
				if n, err := strconv.Atoi(string(m[1])); err == nil && n >= 21 && n <= 40 {
					return n
				}
			}
		}
	}
	return 36
}

// ensurePlatformInstalled installs the Android platform (and
// matching build-tools) for api when it is missing from an
// otherwise existing SDK. A no-op when the platform is already
// there or when there is no sdkmanager to install it with —
// Gradle then reports the missing platform itself.
func ensurePlatformInstalled(dir string, api int) {
	platformDir := filepath.Join(dir, "platforms", fmt.Sprintf("android-%d", api))
	if st, err := os.Stat(platformDir); err == nil && st.IsDir() {
		return
	}
	sdkman := sdkmanagerPath(dir)
	if !fileExists(sdkman) {
		return
	}
	stepLine(fmt.Sprintf("Android platform android-%d not installed — installing it…", api))
	pkgs := []string{fmt.Sprintf("platforms;android-%d", api), fmt.Sprintf("build-tools;%d.0.0", api)}
	args := append([]string{"--sdk_root=" + dir}, pkgs...)
	if err := runSdkmanagerYes(sdkman, args...); err != nil {
		die("could not install Android platform " + strconv.Itoa(api) + " (" + err.Error() + "). Install it with sdkmanager and retry.")
	}
	okLine(fmt.Sprintf("Android platform android-%d ready", api))
}

// ensureAndroidSdk makes sure the Android SDK exists: an existing
// SDK is used as-is, otherwise it is installed once. It sets
// ANDROID_HOME / ANDROID_SDK_ROOT + PATH for the child processes.
// Call it after ensureJava — sdkmanager needs Java.
//
// One exception: cmdline-tools 23.0's sdkmanager is only a shim that
// downloads the new Android CLI, and that CLI fails with "Failed to
// connect to remote repository" here — so a 23.0 cmdline-tools is
// replaced with 17.0 (the last classic sdkmanager), keeping the
// already-installed platforms and build-tools.
func ensureAndroidSdk(root string) {
	if dir := findAndroidSdk(); dir != "" {
		if cmdlineToolsIsShim(dir) {
			stepLine("This SDK's cmdline-tools uses the new Android CLI, which cannot reach its repository — replacing it with cmdline-tools 17.0 (keeping your installed platforms)…")
			_ = os.RemoveAll(filepath.Join(dir, "cmdline-tools", "latest"))
			if err := installAndroidSdk(dir, sdkApiLevel(root)); err != nil {
				die("could not install the Android SDK (" + err.Error() + "). Install Android Studio once yourself, or set ANDROID_HOME to an existing SDK and retry.")
			}
		}
		// The SDK exists, but the platform this project needs may
		// not: a project created with a newer BOM needs a newer
		// compileSdk, and its platform is installed here instead
		// of failing later in Gradle.
		ensurePlatformInstalled(dir, sdkApiLevel(root))
		applySdkEnv(dir)
		return
	}
	dir := defaultSdkDir()
	api := sdkApiLevel(root)
	stepLine(fmt.Sprintf("Android SDK not found — installing it once to %s (cmdline-tools + platform-tools + android-%d + build-tools, ~350 MB)…", dir, api))
	if err := installAndroidSdk(dir, api); err != nil {
		die("could not install the Android SDK (" + err.Error() + "). Install Android Studio once yourself, or set ANDROID_HOME to an existing SDK and retry.")
	}
	applySdkEnv(dir)
	okLine("Android SDK ready: " + dir)
}

// cmdlineToolsIsShim reports whether the SDK's cmdline-tools is the
// 23.0+ shim: the new Android CLI ships an `android` binary next to
// sdkmanager, which classic cmdline-tools never had.
func cmdlineToolsIsShim(sdkDir string) bool {
	return fileExists(filepath.Join(sdkDir, "cmdline-tools", "latest", "bin", "android"))
}

func applySdkEnv(dir string) {
	os.Setenv("ANDROID_HOME", dir)
	os.Setenv("ANDROID_SDK_ROOT", dir)
	addToPath(filepath.Join(dir, "platform-tools"))
	addToPath(filepath.Join(dir, "cmdline-tools", "latest", "bin"))
}

// installAndroidSdk downloads cmdline-tools 17.0 (build 13114758)
// and uses its sdkmanager to install the packages for the given API
// level. 17.0 is pinned on purpose: 23.0's sdkmanager is only a shim
// for the new Android CLI, which fails to reach its repository.
func installAndroidSdk(dir string, api int) error {
	zipName := map[string]string{
		"linux":   "commandlinetools-linux-13114758_latest.zip",
		"darwin":  "commandlinetools-mac-13114758_latest.zip",
		"windows": "commandlinetools-win-13114758_latest.zip",
	}[runtime.GOOS]
	if zipName == "" {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	url := "https://dl.google.com/android/repository/" + zipName
	dlDir := filepath.Join(dir, ".skoll-downloads")
	if err := os.MkdirAll(dlDir, 0755); err != nil {
		return err
	}
	zipPath := filepath.Join(dlDir, zipName)
	if err := downloadFileProgress(url, zipPath, "cmdline-tools 17.0"); err != nil {
		return err
	}
	// The zip holds a top-level cmdline-tools/ dir; extract it to a
	// temp dir, then move that dir to cmdline-tools/latest.
	tmpDir := filepath.Join(dir, "cmdline-tools", "tmp-extract")
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}
	if err := unzip(zipPath, tmpDir); err != nil {
		return err
	}
	extracted := filepath.Join(tmpDir, "cmdline-tools")
	if st, err := os.Stat(extracted); err != nil || !st.IsDir() {
		return fmt.Errorf("cmdline-tools zip did not contain a cmdline-tools/ folder")
	}
	latest := filepath.Join(dir, "cmdline-tools", "latest")
	_ = os.RemoveAll(latest)
	if err := os.MkdirAll(filepath.Join(dir, "cmdline-tools"), 0755); err != nil {
		return err
	}
	if err := os.Rename(extracted, latest); err != nil {
		return err
	}
	_ = os.RemoveAll(tmpDir)
	sdkman := sdkmanagerPath(dir)
	if runtime.GOOS != "windows" {
		_ = os.Chmod(sdkman, 0755)
	}
	// sdkmanager installs the packages (licenses auto-accepted).
	sp := startSpinner(fmt.Sprintf("Installing android-%d + build-tools + platform-tools", api))
	pkgs := []string{"platform-tools", fmt.Sprintf("platforms;android-%d", api), fmt.Sprintf("build-tools;%d.0.0", api)}
	args := append([]string{"--sdk_root=" + dir}, pkgs...)
	if err := runSdkmanagerYes(sdkman, args...); err != nil {
		sp.stopFail("sdkmanager install failed")
		return err
	}
	sp.stop("Android SDK packages ready")
	return nil
}

// runSdkmanagerYes runs sdkmanager with "y" fed to stdin for every
// license prompt (the `yes | sdkmanager` equivalent, portable to
// Windows where there is no `yes` command).
func runSdkmanagerYes(sdkman string, args ...string) error {
	cmd := exec.Command(sdkman, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		for i := 0; i < 200; i++ {
			if _, err := io.WriteString(stdin, "y\n"); err != nil {
				return
			}
		}
	}()
	return cmd.Wait()
}
