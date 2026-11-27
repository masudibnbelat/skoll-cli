// Location: sdk.go
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
	"strings"
)

// Android SDK auto-install. An existing SDK is used as-is; otherwise
// cmdline-tools 20.0 is downloaded once and sdkmanager installs
// platform-tools + the platform + build-tools for the project's API
// level. Nothing here needs Android Studio.
//
// A platform is named by its full spec: "36", "37.0" or "37.2".
// Android now ships minor API releases, and the libraries of the
// latest Compose BOM (2026.11.01) need compileSdk 37.1 at least —
// plain `compileSdk = 37` (platform 37.0) fails their AAR metadata
// check, so new projects use 37.2. The Gradle syntax for that is
// the compileSdk block (see versions.go compileSdkBlock):
// compileSdk { version = release(37) { minorApiLevel = 2 } }.

// cmdline-tools 20.0 (build 14742923) is the minimum that understands
// Android 37. The old pinned tools (build 13114758, labelled 17.0)
// only understand SDK XML up to v3, so they cannot even see the new
// packages and fail with "Failed to find package
// 'platforms;android-37'". 20.0 is also the newest classic
// sdkmanager: it has no `android` binary, so it is not the 23.0 shim
// that could not reach its repository, and it parses the Major.Minor
// package names (android-37.0, android-37.2) correctly. The zip
// exists for all three OSes under the same build number (verified
// 2026-11-27).
const cmdlineToolsBuild = "14742923"
const cmdlineToolsVersion = "20.0"

// winBatCmd builds the command to run a program on Windows the way
// cmd.exe needs it. A .bat/.cmd file (sdkmanager.bat, gradlew.bat,
// flutter.bat) runs through cmd.exe, and cmd's /c parsing strips the
// outermost quotes — so a path with a space in it ("C:\Users\Royal
// Academy\...") is split at the space and fails with
// "'C:\Users\Royal' is not recognized as an internal or external
// command". Wrapping the whole command in one more pair of quotes
// keeps the quoted .bat path and any space-containing argument
// intact. Non-Windows and non-.bat programs run exactly as before.
func winBatCmd(name string, args ...string) *exec.Cmd {
	if runtime.GOOS != "windows" {
		return exec.Command(name, args...)
	}
	prog := name
	if p, err := exec.LookPath(name); err == nil {
		prog = p
	}
	if !strings.EqualFold(filepath.Ext(prog), ".bat") && !strings.EqualFold(filepath.Ext(prog), ".cmd") {
		return exec.Command(name, args...)
	}
	var b strings.Builder
	b.WriteString(quoteWinArg(prog))
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(quoteWinArg(a))
	}
	return exec.Command("cmd.exe", "/c", `"`+b.String()+`"`)
}

// quoteWinArg quotes one command-line argument for cmd.exe: wrap in
// double quotes when it contains a space, tab or quote.
func quoteWinArg(s string) string {
	if strings.ContainsAny(s, " \t\"") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

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

// readAppBuildGradle reads the project's app/build.gradle.kts.
// nil when the file is missing (not a Gradle project, or Flutter).
func readAppBuildGradle(root string) []byte {
	for _, name := range []string{
		filepath.Join("app", "build.gradle.kts"),
		filepath.Join("app", "build.gradle"),
	} {
		if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			return data
		}
	}
	return nil
}

// sdkApiLevel reads the project's compileSdk major version:
// app/build.gradle.kts, either `compileSdk = 36` or the block form
// `compileSdk { version = release(37) { minorApiLevel = 2 } }`.
// Flutter projects use flutter.compileSdkVersion, which is 36, so
// they fall back to 36 too.
func sdkApiLevel(root string) int {
	data := readAppBuildGradle(root)
	if data == nil {
		return 36
	}
	if m := regexp.MustCompile(`release\((\d+)\)`).FindSubmatch(data); m != nil {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n >= 21 && n <= 40 {
			return n
		}
	}
	if m := regexp.MustCompile(`compileSdk(?:Version)?\s*=?\s*(\d+)`).FindSubmatch(data); m != nil {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n >= 21 && n <= 40 {
			return n
		}
	}
	return 36
}

// sdkPlatformSpec reads the project's full compileSdk spec, e.g.
// "37.2" for the block form, "36" for `compileSdk = 36`. "36" when
// the file has no compileSdk at all.
func sdkPlatformSpec(root string) string {
	data := readAppBuildGradle(root)
	if data == nil {
		return "36"
	}
	if m := regexp.MustCompile(`release\((\d+)\)`).FindSubmatch(data); m != nil {
		major := string(m[1])
		if mm := regexp.MustCompile(`minorApiLevel\s*=\s*(\d+)`).FindSubmatch(data); mm != nil {
			return major + "." + string(mm[1])
		}
		return major
	}
	if m := regexp.MustCompile(`compileSdk(?:Version)?\s*=?\s*(\d+)`).FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return "36"
}

// specMajor is the major API number of a spec, e.g. 37 for "37.2".
func specMajor(spec string) int {
	n, _ := strconv.Atoi(strings.Split(spec, ".")[0])
	return n
}

// buildToolsFor is the build-tools version for a spec: the major
// number with .0.0 — build-tools do not track platform minors, so
// 37.2 still uses build-tools 37.0.0.
func buildToolsFor(spec string) string {
	return strconv.Itoa(specMajor(spec)) + ".0.0"
}

// platformPackage is the exact sdkmanager package name for a spec.
// From API 37 Android publishes platforms with a minor version —
// platforms;android-37.2, NOT platforms;android-37. Asking for the
// old name fails with "Failed to find package".
func platformPackage(spec string) string {
	return "platforms;android-" + spec
}

// platformLabel is the short name shown to the user, e.g. android-37.2.
func platformLabel(spec string) string {
	return "android-" + spec
}

// platformDirExists reports whether the platform for spec is already
// installed. For "37.0" it also accepts the plain android-37 folder
// (created by the old symlink step).
func platformDirExists(dir string, spec string) bool {
	names := []string{"android-" + spec}
	if spec == "37.0" {
		names = append(names, "android-37")
	}
	for _, n := range names {
		if st, err := os.Stat(filepath.Join(dir, "platforms", n)); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

// linkPlatform37 makes Gradle/AGP find the 37.0 platform as
// android-37: sdkmanager installs it as platforms/android-37.0,
// but a plain `compileSdk = 37` still resolves the android-37
// folder. Best effort only — on Windows a symlink needs privileges,
// and newer AGP may resolve 37.0 directly, so failures are ignored.
func linkPlatform37(dir string) {
	src := filepath.Join(dir, "platforms", "android-37.0")
	dst := filepath.Join(dir, "platforms", "android-37")
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return
	}
	if _, err := os.Lstat(dst); err == nil {
		return
	}
	_ = os.Symlink("android-37.0", dst)
}

// cmdlineToolsRevision reads Pkg.Revision from the installed
// cmdline-tools, e.g. "19.0" or "20.0". "" when it cannot be read.
func cmdlineToolsRevision(sdkDir string) string {
	data, err := os.ReadFile(filepath.Join(sdkDir, "cmdline-tools", "latest", "source.properties"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Pkg.Revision=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Pkg.Revision="))
		}
	}
	return ""
}

// cmdlineToolsMajor is the major version number, e.g. 20 for "20.0".
// 0 when the revision cannot be read.
func cmdlineToolsMajor(sdkDir string) int {
	rev := cmdlineToolsRevision(sdkDir)
	if rev == "" {
		return 0
	}
	n, _ := strconv.Atoi(strings.Split(rev, ".")[0])
	return n
}

// cmdlineToolsTooOld reports whether the installed cmdline-tools is
// known to be older than 20.0, i.e. it cannot parse Android 37's
// Major.Minor packages. Unknown revisions return false — the
// install-failure retry in ensurePlatformInstalled covers that case.
func cmdlineToolsTooOld(sdkDir string) bool {
	major := cmdlineToolsMajor(sdkDir)
	return major != 0 && major < 20
}

// ensurePlatformInstalled installs the Android platform (and
// matching build-tools) for spec when it is missing from an
// otherwise existing SDK. It never leaves the user with a dead
// project: if the latest platform cannot be installed even after
// upgrading cmdline-tools, the project is downgraded to the last
// verified set (compileSdk 36 + pinned BOM) and that platform is
// installed instead. A no-op when the platform is already there or
// when there is no sdkmanager to install it with — Gradle then
// reports the missing platform itself.
func ensurePlatformInstalled(dir string, spec string, root string) {
	if platformDirExists(dir, spec) {
		linkPlatform37(dir)
		return
	}
	sdkman := sdkmanagerPath(dir)
	if !fileExists(sdkman) {
		return
	}
	stepLine(fmt.Sprintf("Android platform %s not installed — installing it…", platformLabel(spec)))
	upgraded := false
	// API 37 needs cmdline-tools 20.0+. Upgrade first when the
	// installed tools are known to be too old, instead of letting
	// sdkmanager fail with "Failed to find package".
	if specMajor(spec) >= 37 && cmdlineToolsTooOld(dir) {
		stepLine(fmt.Sprintf("This SDK's cmdline-tools %s is too old to see Android 37 — upgrading to cmdline-tools %s (keeping your installed platforms)…", cmdlineToolsRevision(dir), cmdlineToolsVersion))
		if err := upgradeCmdlineTools(dir); err != nil {
			die("could not upgrade cmdline-tools to " + cmdlineToolsVersion + " (" + err.Error() + "). Install Android Studio once yourself, or set ANDROID_HOME to an existing SDK and retry.")
		}
		upgraded = true
	}
	if err := installPlatformPackages(dir, spec); err == nil {
		linkPlatform37(dir)
		okLine(fmt.Sprintf("Android platform %s ready", platformLabel(spec)))
		return
	}
	// First install failed. If the revision could not be read, the
	// check above could not see old tools — upgrade once and retry.
	if specMajor(spec) >= 37 && !upgraded {
		stepLine("Platform install failed — the installed cmdline-tools may be too old to see Android 37, upgrading to cmdline-tools " + cmdlineToolsVersion + " and retrying…")
		if err := upgradeCmdlineTools(dir); err == nil {
			if err := installPlatformPackages(dir, spec); err == nil {
				linkPlatform37(dir)
				okLine(fmt.Sprintf("Android platform %s ready", platformLabel(spec)))
				return
			}
		}
	}
	// The latest platform is not installable on this machine. Fall
	// back to the verified set instead of dying: downgrade the
	// project (compileSdk 36 + pinned BOM, Gradle/AGP/Kotlin stay
	// at the latest versions) and install that platform.
	if specMajor(spec) >= 37 && downgradeProjectToVerified(root) {
		stepLine(fmt.Sprintf("Android 37 is not installable here, so this project was switched to the last verified set (compileSdk 36, Compose BOM %s) — Gradle, AGP and Kotlin stay at the latest versions.", pinnedVersions.bom))
		if err := installPlatformPackages(dir, fallbackCompileSdk()); err == nil {
			okLine("Android platform android-36 ready (fallback set)")
			return
		}
	}
	die("could not install Android platform " + platformLabel(spec) + ". Try it yourself: yes | \"" + sdkman + "\" --sdk_root=\"" + dir + "\" \"" + platformPackage(spec) + "\" \"build-tools;" + buildToolsFor(spec) + "\"")
}

// installPlatformPackages installs one platform + its build-tools
// with sdkmanager (licenses auto-accepted).
func installPlatformPackages(dir string, spec string) error {
	sdkman := sdkmanagerPath(dir)
	pkgs := []string{platformPackage(spec), "build-tools;" + buildToolsFor(spec)}
	args := append([]string{"--sdk_root=" + dir}, pkgs...)
	return runSdkmanagerYes(sdkman, args...)
}

// downgradeProjectToVerified rewrites a generated project to the last
// verified set: compileSdk 36 and the pinned Compose BOM. It handles
// both the plain form (`compileSdk = 37`) and the block form
// (`compileSdk { version = release(37) { minorApiLevel = 2 } }`).
// It returns true when it changed app/build.gradle.kts.
func downgradeProjectToVerified(root string) bool {
	path := filepath.Join(root, "app", "build.gradle.kts")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	orig := string(data)
	changed := regexp.MustCompile(`(?s)compileSdk\s*\{\s*version\s*=\s*release\(\d+\)\s*\{\s*minorApiLevel\s*=\s*\d+\s*\}\s*\}`).ReplaceAllString(orig, "compileSdk = 36")
	changed = regexp.MustCompile(`compileSdk\s*=\s*\d+`).ReplaceAllString(changed, "compileSdk = 36")
	changed = regexp.MustCompile(`androidx\.compose:compose-bom:[0-9.]+`).ReplaceAllString(changed, "androidx.compose:compose-bom:"+pinnedVersions.bom)
	if changed == orig {
		return false
	}
	if err := os.WriteFile(path, []byte(changed), 0644); err != nil {
		return false
	}
	return true
}

// upgradeCmdlineTools replaces cmdline-tools/latest with 20.0,
// keeping every installed platform and build-tools.
func upgradeCmdlineTools(dir string) error {
	_ = os.RemoveAll(filepath.Join(dir, "cmdline-tools", "latest"))
	return installCmdlineTools(dir)
}

// ensureAndroidSdk makes sure the Android SDK exists: an existing
// SDK is used as-is, otherwise it is installed once. It sets
// ANDROID_HOME / ANDROID_SDK_ROOT + PATH for the child processes.
// Call it after ensureJava — sdkmanager needs Java.
//
// One exception: cmdline-tools 23.0's sdkmanager is only a shim that
// downloads the new Android CLI, and that CLI fails with "Failed to
// connect to remote repository" here — so a 23.0 cmdline-tools is
// replaced with 20.0 (the last classic sdkmanager that understands
// Android 37), keeping the already-installed platforms and
// build-tools. Note 22.0 also ships an `android` binary, but it is
// the real bundled CLI, not the shim — it is left alone.
func ensureAndroidSdk(root string) {
	if dir := findAndroidSdk(); dir != "" {
		if cmdlineToolsIsShim(dir) {
			stepLine("This SDK's cmdline-tools uses the new Android CLI, which cannot reach its repository — replacing it with cmdline-tools " + cmdlineToolsVersion + " (keeping your installed platforms)…")
			_ = os.RemoveAll(filepath.Join(dir, "cmdline-tools", "latest"))
			if err := installAndroidSdk(dir, sdkPlatformSpec(root)); err != nil {
				die("could not install the Android SDK (" + err.Error() + "). Install Android Studio once yourself, or set ANDROID_HOME to an existing SDK and retry.")
			}
		}
		// The SDK exists, but the platform this project needs may
		// not: a project created with a newer BOM needs a newer
		// compileSdk, and its platform is installed here instead
		// of failing later in Gradle.
		ensurePlatformInstalled(dir, sdkPlatformSpec(root), root)
		applySdkEnv(dir)
		return
	}
	dir := defaultSdkDir()
	spec := sdkPlatformSpec(root)
	stepLine(fmt.Sprintf("Android SDK not found — installing it once to %s (cmdline-tools + platform-tools + %s + build-tools, ~350 MB)…", dir, platformLabel(spec)))
	if err := installAndroidSdk(dir, spec); err != nil {
		die("could not install the Android SDK (" + err.Error() + "). Install Android Studio once yourself, or set ANDROID_HOME to an existing SDK and retry.")
	}
	applySdkEnv(dir)
	okLine("Android SDK ready: " + dir)
}

// cmdlineToolsIsShim reports whether the SDK's cmdline-tools is the
// 23.0+ shim: the new Android CLI ships an `android` binary next to
// sdkmanager, which classic cmdline-tools never had. 22.0 also ships
// that binary, so the revision is checked when it can be read.
func cmdlineToolsIsShim(sdkDir string) bool {
	if !fileExists(filepath.Join(sdkDir, "cmdline-tools", "latest", "bin", "android")) {
		return false
	}
	if major := cmdlineToolsMajor(sdkDir); major != 0 {
		return major >= 23
	}
	return true
}

func applySdkEnv(dir string) {
	os.Setenv("ANDROID_HOME", dir)
	os.Setenv("ANDROID_SDK_ROOT", dir)
	addToPath(filepath.Join(dir, "platform-tools"))
	addToPath(filepath.Join(dir, "cmdline-tools", "latest", "bin"))
}

// installCmdlineTools downloads cmdline-tools 20.0 (build 14742923)
// and extracts it to cmdline-tools/latest. It installs no packages.
func installCmdlineTools(dir string) error {
	zipName := map[string]string{
		"linux":   "commandlinetools-linux-" + cmdlineToolsBuild + "_latest.zip",
		"darwin":  "commandlinetools-mac-" + cmdlineToolsBuild + "_latest.zip",
		"windows": "commandlinetools-win-" + cmdlineToolsBuild + "_latest.zip",
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
	if err := downloadFileProgress(url, zipPath, "cmdline-tools "+cmdlineToolsVersion); err != nil {
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
	return nil
}

// installAndroidSdk downloads cmdline-tools 20.0 (build 14742923)
// and uses its sdkmanager to install the packages for the given API
// level. 20.0 is pinned on purpose: older tools cannot see Android
// 37's packages, and 23.0's sdkmanager is only a shim for the new
// Android CLI, which fails to reach its repository.
func installAndroidSdk(dir string, spec string) error {
	if err := installCmdlineTools(dir); err != nil {
		return err
	}
	// sdkmanager installs the packages (licenses auto-accepted).
	sp := startSpinner(fmt.Sprintf("Installing %s + build-tools + platform-tools", platformPackage(spec)))
	pkgs := []string{"platform-tools", platformPackage(spec), "build-tools;" + buildToolsFor(spec)}
	args := append([]string{"--sdk_root=" + dir}, pkgs...)
	if err := runSdkmanagerYes(sdkmanagerPath(dir), args...); err != nil {
		sp.stopFail("sdkmanager install failed")
		return err
	}
	linkPlatform37(dir)
	sp.stop("Android SDK packages ready")
	return nil
}

// runSdkmanagerYes runs sdkmanager with "y" fed to stdin for every
// license prompt (the `yes | sdkmanager` equivalent, portable to
// Windows where there is no `yes` command).
func runSdkmanagerYes(sdkman string, args ...string) error {
	cmd := winBatCmd(sdkman, args...)
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
