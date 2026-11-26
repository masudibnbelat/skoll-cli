package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Flags for `skoll build` and `skoll run`, bound in init() below.
var (
	buildRelease bool
	buildClean   bool
	buildBundle  bool
	runClean     bool
	runRelease   bool
	runDevice    string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the APK, no install",
	Long: `Build only — never install. assembleDebug needs no phone and no adb.
The APK is copied to <project>/apk/<project>-debug.apk (or -release.apk).
For a Flutter project it runs ` + "`flutter build apk`" + ` instead.
For a library project it builds the .aar instead (to <project>/aar/).
A release build is signed: the release keystore is created once if
needed (see skoll keystore).
With --bundle it builds the App Bundle (.aab) for Google Play
instead, to <project>/bundle/<project>-release.aab
(` + "`flutter build appbundle`" + ` for Flutter). A library project
has no bundle.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdBuild(buildRelease, buildClean, buildBundle)
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Build, install and launch the app on the connected phone",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdRun(runClean, runRelease, runDevice)
	},
}

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List connected devices (adb devices)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdDevices()
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Follow logcat for this app (Ctrl+C to stop)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdLogs()
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall this app from the connected phone",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdUninstall()
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Run the Gradle clean task",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdClean()
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the environment: Java, adb, Gradle, project",
	Long: `Doctor only reports — it never downloads anything.
The install happens on the first build / run.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdDoctor()
	},
}

func init() {
	buildCmd.Flags().BoolVar(&buildRelease, "release", false, "Build the release APK (assembleRelease)")
	buildCmd.Flags().BoolVar(&buildClean, "clean", false, "Run the clean task first")
	buildCmd.Flags().BoolVar(&buildBundle, "bundle", false, "Build the App Bundle (.aab) for Google Play instead of the APK")
	runCmd.Flags().BoolVar(&runClean, "clean", false, "Run the clean task first")
	runCmd.Flags().BoolVar(&runRelease, "release", false, "Build and install the release APK (installRelease)")
	runCmd.Flags().StringVar(&runDevice, "device", "", "Only use the device with this serial")
}

func gradleArgs() []string {
	// Kept on purpose: --no-daemon --max-workers=1 (8 GB RAM).
	return []string{"--no-daemon", "--max-workers=1"}
}

func mustRoot() string {
	root, err := findProjectRoot()
	if err != nil {
		die("no Android project found above this folder.")
	}
	return root
}

func mustGradle(root string) string {
	g, err := pickGradle(root)
	if err != nil {
		die(err.Error())
	}
	return g
}

func cmdDoctor() {
	fmt.Println("skoll " + version + " — environment check")
	fmt.Println("ANDROID_HOME: " + os.Getenv("ANDROID_HOME"))
	// Java and adb are reported from where skoll would actually use
	// them (system first, skoll cache second). Doctor never downloads
	// anything — the install happens on the first build / run.
	if bin, _, src := findJava(); bin != "" {
		doctorLine("java", bin+" ("+src+")", true)
	} else {
		doctorLine("java", "MISSING (auto-installs Temurin 17 on build)", false)
	}
	if p, src := findAdb(); p != "" {
		doctorLine("adb", p+" ("+src+")", true)
	} else {
		doctorLine("adb", "MISSING (auto-installs platform-tools on run)", false)
	}
	if dir := findAndroidSdk(); dir != "" {
		doctorLine("android-sdk", dir, true)
	} else {
		doctorLine("android-sdk", "MISSING (auto-installs on build)", false)
	}
	if p, src := findFlutter(); p != "" {
		doctorLine("flutter", p+" ("+src+")", true)
	} else {
		doctorLine("flutter", "MISSING (auto-installs on Flutter build)", false)
	}
	// Release readiness: git + gh login, and the release keystore.
	// Doctor never downloads anything — the keystore is created on
	// the first release build, not here.
	if p, err := lookPath("git"); err == nil {
		doctorLine("git", p, true)
	} else {
		doctorLine("git", "MISSING (needed for `skoll release`)", false)
	}
	if p, err := lookPath("gh"); err == nil {
		if _, err := runCapture("gh", "auth", "status"); err == nil {
			doctorLine("gh", p+" (logged in)", true)
		} else {
			doctorLine("gh", p+" (NOT logged in — run: gh auth login)", false)
		}
	} else {
		doctorLine("gh", "MISSING (needed for `skoll release`)", false)
	}
	if _, err := lookPath("gradle"); err == nil {
		doctorLine("gradle", "on PATH (only the wrapper version is used)", true)
	} else {
		doctorLine("gradle", "not on PATH (fine — the wrapper version is used)", true)
	}
	if ksPath := releaseKeystorePath(); true {
		if _, err := os.Stat(ksPath); err == nil {
			doctorLine("keystore", ksPath+" (alias release)", true)
		} else {
			doctorLine("keystore", "MISSING (created on the first release build — back it up)", false)
		}
	}
	// Free space on the home disk, where the SDK, Gradle, Flutter
	// and the keystore all live. A first build needs ~2 GB; warn
	// below that instead of failing halfway through a download.
	if home, err := os.UserHomeDir(); err == nil {
		if free, ok := freeDiskSpace(home); ok {
			if free < 2*1024*1024*1024 {
				doctorLine("disk", "only "+formatBytes(int64(free))+" free on the home disk — a first build needs ~2 GB", false)
			} else {
				doctorLine("disk", formatBytes(int64(free))+" free on the home disk", true)
			}
		}
	}
	fmt.Println("---")
	adbBin, _ := findAdb()
	if adbBin == "" {
		adbBin = "adb"
	}
	if out, err := runCapture(adbBin, "devices"); err == nil {
		fmt.Print(out)
	} else {
		fmt.Println("adb: no devices / adb not running")
	}
	root, err := findProjectRoot()
	if err != nil {
		fmt.Println("Project: none found above " + mustCwd())
		return
	}
	fmt.Println("Project: " + root)
	if isFlutterProject(root) {
		fmt.Println("Project type: Flutter")
		if v := wrapperGradleVersion(filepath.Join(root, "android")); v != "" {
			fmt.Println("Android Gradle version: " + v)
		}
		doctorSigning(root, true)
		return
	}
	if isLibraryProject(root) {
		fmt.Println("Project type: Android library (.aar)")
		fmt.Println("Release signing: not needed (a library is not installed)")
	} else {
		fmt.Println("Project type: Kotlin (Gradle)")
		doctorSigning(root, false)
	}
	if v := wrapperGradleVersion(root); v != "" {
		fmt.Println("Wrapper Gradle version: " + v)
	} else {
		fmt.Println("Wrapper Gradle version: not found in gradle-wrapper.properties")
	}
	// Read-only lookup: doctor must never download Gradle.
	if g := findGradleReadOnly(root); g != "" {
		fmt.Println("Gradle: " + g)
	} else {
		fmt.Println("Gradle: none installed (auto-downloads on build)")
	}
}

func mustCwd() string {
	d, _ := os.Getwd()
	return d
}

// doctorSigning reports whether this project can sign a release
// build: it reads the signing keys (local.properties for Kotlin,
// android/key.properties for Flutter) and checks the keystore file
// they point at still exists. A project with no keys is fine — the
// first release build creates the shared keystore and writes them.
func doctorSigning(root string, flutter bool) {
	var propsPath, storeKey string
	if flutter {
		propsPath = filepath.Join(root, "android", "key.properties")
		storeKey = "storeFile"
	} else {
		propsPath = filepath.Join(root, "local.properties")
		storeKey = "releaseStoreFile"
	}
	props := readPropsFile(propsPath)
	store := props[storeKey]
	if store == "" {
		fmt.Println("Release signing: not configured yet (first release build creates the shared keystore)")
		return
	}
	if _, err := os.Stat(store); err == nil {
		fmt.Println("Release signing: configured — " + store)
	} else {
		fmt.Println("Release signing: BROKEN — keystore file is missing: " + store + " (restore it, or delete the keys from " + filepath.Base(propsPath) + " so a new one is created)")
	}
}

func cmdDevices() {
	ensureAdb()
	if err := runLive("adb", "devices"); err != nil {
		die("adb devices failed: " + err.Error())
	}
}

func cmdBuild(release, clean, bundle bool) {
	// Build only — never install. assembleDebug needs no phone and no adb.
	// (installDebug would fail with "No connected devices!" when no phone
	// is plugged in, so it belongs to `skoll run` only, not to `skoll build`.)
	start := time.Now()
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	if isFlutterProject(root) {
		buildFlutter(root, release, clean, bundle)
		return
	}
	if isLibraryProject(root) {
		if bundle {
			die(filepath.Base(root) + " is a library project — it builds an .aar, not an App Bundle.")
		}
		buildLibrary(root, release, clean)
		return
	}
	task := "assembleDebug"
	variant := "debug"
	if release {
		task = "assembleRelease"
		variant = "release"
	}
	if bundle {
		task = "bundleDebug"
		if release {
			task = "bundleRelease"
		}
	}
	say("Project: " + root)
	ensureJava()
	ensureAndroidSdk(root)
	if release {
		// A release APK must be signed to install — make sure the
		// keystore exists before the build, not after.
		ensureReleaseKeystore(root)
	}
	gradle := mustGradle(root)
	say("Gradle: " + gradle)
	all := append(gradleArgs(), task)
	if clean {
		if err := runLive(gradle, append(gradleArgs(), "clean")...); err != nil {
			die("clean failed: " + err.Error())
		}
	}
	kind := "APK"
	if bundle {
		kind = "App Bundle"
	}
	stepLine("Building " + filepath.Base(root) + " " + kind + " (" + variant + ")…")
	if err := runLive(gradle, all...); err != nil {
		die("build failed: " + err.Error())
	}
	if bundle {
		// The .aab is signed with the same signing config as the
		// APK, so there is no separate signature check here — the
		// APK check in `skoll build --release` / `skoll
		// release` is what proves the config works.
		dest, err := copyBuiltBundle(root, variant)
		if err != nil {
			die(err.Error())
		}
		printAabSummary(dest, start)
		return
	}
	// Copy the built APK to <project>/apk/<project>-debug.apk (or
	// -release.apk), so it is easy to find and share — no digging
	// through app/build/outputs/apk/. Copy, not move: the original
	// stays where Gradle put it. A release build is also checked:
	// an unsigned (or debug-signed) release APK cannot be installed,
	// so it fails here instead of silently landing in apk/.
	dest, err := copyBuiltApk(root, variant)
	if err != nil {
		die(err.Error())
	}
	printApkSummary(dest, start)
	if release {
		if err := verifyApkSigned(dest); err != nil {
			die(err.Error())
		}
	}
}

// printAabSummary prints the finished line for a built App Bundle:
// ✓ AAB ready: bundle/moan-release.aab (3.1 MB) in 1m 05s
func printAabSummary(dest string, start time.Time) {
	okLine("AAB ready: " + dest + apkSizeSuffix(dest) + " in " + formatDuration(time.Since(start)))
}

// copyBuiltBundle finds the .aab Gradle just built
// (app/build/outputs/bundle/<variant>/*.aab), creates <root>/bundle/
// if needed, and copies the .aab there renamed to
// <project>-<variant>.aab. The original stays in place. It returns
// the destination path. Errors are returned, not died on, so a
// caller after a release tag can clean the tag up first.
func copyBuiltBundle(root, variant string) (string, error) {
	bundleDir := filepath.Join(root, "app", "build", "outputs", "bundle", variant)
	var src string
	std := filepath.Join(bundleDir, "app-"+variant+".aab")
	if _, err := os.Stat(std); err == nil {
		src = std
	} else {
		entries, _ := os.ReadDir(bundleDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".aab") {
				src = filepath.Join(bundleDir, e.Name())
				break
			}
		}
	}
	if src == "" {
		return "", fmt.Errorf("build passed, but no .aab was found in %s", bundleDir)
	}
	outDir := filepath.Join(root, "bundle")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create bundle folder: %v", err)
	}
	dest := filepath.Join(outDir, filepath.Base(root)+"-"+variant+".aab")
	if err := copyFile(src, dest); err != nil {
		return "", fmt.Errorf("could not copy AAB to %s: %v", dest, err)
	}
	return dest, nil
}

// apkSizeSuffix renders " (4.2 MB)" for a built APK, or "" when the
// file cannot be read.
func apkSizeSuffix(dest string) string {
	if st, err := os.Stat(dest); err == nil {
		return " (" + formatBytes(st.Size()) + ")"
	}
	return ""
}

// printApkSummary prints the finished line for a built APK:
// ✓ APK ready: apk/moan-debug.apk (4.2 MB) in 1m 23s
func printApkSummary(dest string, start time.Time) {
	okLine("APK ready: " + dest + apkSizeSuffix(dest) + " in " + formatDuration(time.Since(start)))
}

// copyBuiltApk finds the APK Gradle just built (app/build/outputs/apk/
// <variant>/*.apk), creates <root>/apk/ if needed, and copies the APK
// there renamed to <project>-<variant>.apk, e.g. MyApp-debug.apk.
// The original file stays in place. An existing file with the same
// name is replaced. It returns the destination path. Errors are
// returned, not died on, so a caller after a release tag can clean
// the tag up first.
func copyBuiltApk(root, variant string) (string, error) {
	apkDir := filepath.Join(root, "app", "build", "outputs", "apk", variant)
	var src string
	// Prefer the standard app-<variant>.apk name, else any .apk here.
	std := filepath.Join(apkDir, "app-"+variant+".apk")
	if _, err := os.Stat(std); err == nil {
		src = std
	} else {
		entries, _ := os.ReadDir(apkDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".apk") {
				src = filepath.Join(apkDir, e.Name())
				break
			}
		}
	}
	if src == "" {
		return "", fmt.Errorf("build passed, but no APK was found in %s", apkDir)
	}
	outDir := filepath.Join(root, "apk")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create apk folder: %v", err)
	}
	dest := filepath.Join(outDir, filepath.Base(root)+"-"+variant+".apk")
	if err := copyFile(src, dest); err != nil {
		return "", fmt.Errorf("could not copy APK to %s: %v", dest, err)
	}
	return dest, nil
}

// buildLibrary builds a library project (.aar, no APK, no phone).
func buildLibrary(root string, release, clean bool) {
	start := time.Now()
	task := "assembleDebug"
	variant := "debug"
	if release {
		task = "assembleRelease"
		variant = "release"
	}
	say("Project: " + root + " (library)")
	ensureJava()
	ensureAndroidSdk(root)
	gradle := mustGradle(root)
	say("Gradle: " + gradle)
	if clean {
		if err := runLive(gradle, append(gradleArgs(), "clean")...); err != nil {
			die("clean failed: " + err.Error())
		}
	}
	stepLine("Building library (" + variant + ")…")
	if err := runLive(gradle, append(gradleArgs(), task)...); err != nil {
		die("build failed: " + err.Error())
	}
	dest, err := copyBuiltAar(root, variant)
	if err != nil {
		die(err.Error())
	}
	printAarSummary(dest, start)
}

// printAarSummary prints the finished line for a built library:
// ✓ AAR ready: aar/mylib-release.aar (123 KB) in 4s
func printAarSummary(dest string, start time.Time) {
	okLine("AAR ready: " + dest + apkSizeSuffix(dest) + " in " + formatDuration(time.Since(start)))
}

// copyBuiltAar finds the .aar Gradle just built
// (library/build/outputs/aar/<variant>/*.aar), creates <root>/aar/
// if needed, and copies the .aar there renamed to
// <project>-<variant>.aar. The original stays in place. It returns
// the destination path. Errors are returned, not died on, so a
// caller after a release tag can clean the tag up first.
func copyBuiltAar(root, variant string) (string, error) {
	aarDir := filepath.Join(root, "library", "build", "outputs", "aar", variant)
	var src string
	std := filepath.Join(aarDir, "library-"+variant+".aar")
	if _, err := os.Stat(std); err == nil {
		src = std
	} else {
		entries, _ := os.ReadDir(aarDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".aar") {
				src = filepath.Join(aarDir, e.Name())
				break
			}
		}
	}
	if src == "" {
		return "", fmt.Errorf("build passed, but no .aar was found in %s", aarDir)
	}
	outDir := filepath.Join(root, "aar")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create aar folder: %v", err)
	}
	dest := filepath.Join(outDir, filepath.Base(root)+"-"+variant+".aar")
	if err := copyFile(src, dest); err != nil {
		return "", fmt.Errorf("could not copy AAR to %s: %v", dest, err)
	}
	return dest, nil
}

// buildFlutter builds a Flutter project's APK with the Flutter SDK
// and copies it to <project>/apk/<project>-<variant>.apk, like the
// Gradle path does for Kotlin projects. With bundle=true it builds
// the App Bundle instead, to <project>/bundle/<project>-<variant>.aab.
func buildFlutter(root string, release, clean, bundle bool) {
	start := time.Now()
	ensureJava()
	ensureAndroidSdk(root)
	ensureFlutter()
	if release {
		// So the release APK is signed with the real key, not the
		// debug key the Flutter template falls back to.
		ensureReleaseKeystore(root)
	}
	say("Project: " + root + " (Flutter)")
	if clean {
		if err := runLive("flutter", "clean"); err != nil {
			die("flutter clean failed: " + err.Error())
		}
	}
	variant := "debug"
	mode := "--debug"
	if release {
		variant = "release"
		mode = "--release"
	}
	if bundle {
		stepLine("Building Flutter App Bundle (" + variant + ")…")
		if err := runLive("flutter", "build", "appbundle", mode); err != nil {
			die("flutter build failed: " + err.Error())
		}
		// Same as the Gradle bundle: the .aab is signed with the
		// same config as the APK, so no separate signature check.
		dest, err := copyFlutterBundle(root, variant)
		if err != nil {
			die(err.Error())
		}
		printAabSummary(dest, start)
		return
	}
	stepLine("Building Flutter APK (" + variant + ")…")
	if err := runLive("flutter", "build", "apk", mode); err != nil {
		die("flutter build failed: " + err.Error())
	}
	// A release build is also checked: an unsigned (or debug-signed)
	// release APK cannot be installed, so it fails here instead of
	// silently landing in apk/.
	dest, err := copyFlutterApk(root, variant)
	if err != nil {
		die(err.Error())
	}
	printApkSummary(dest, start)
	if release {
		if err := verifyApkSigned(dest); err != nil {
			die(err.Error())
		}
	}
}

// copyFlutterBundle copies the .aab `flutter build appbundle` just
// made (build/app/outputs/bundle/<variant>/app-<variant>.aab) to
// <project>/bundle/<project>-<variant>.aab. Copy, not move. It
// returns the destination path. Errors are returned, not died on,
// so a caller after a release tag can clean the tag up first.
func copyFlutterBundle(root, variant string) (string, error) {
	src := filepath.Join(root, "build", "app", "outputs", "bundle", variant, "app-"+variant+".aab")
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("build passed, but no .aab was found in %s", filepath.Join(root, "build", "app", "outputs", "bundle", variant))
	}
	outDir := filepath.Join(root, "bundle")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create bundle folder: %v", err)
	}
	dest := filepath.Join(outDir, filepath.Base(root)+"-"+variant+".aab")
	if err := copyFile(src, dest); err != nil {
		return "", fmt.Errorf("could not copy AAB to %s: %v", dest, err)
	}
	return dest, nil
}

// copyFlutterApk copies the APK `flutter build apk` just made
// (build/app/outputs/flutter-apk/app-<variant>.apk) to
// <project>/apk/<project>-<variant>.apk. Copy, not move: the
// original stays where Flutter put it. It returns the destination path.
// Errors are returned, not died on, so a caller after a release tag
// can clean the tag up first.
func copyFlutterApk(root, variant string) (string, error) {
	src := filepath.Join(root, "build", "app", "outputs", "flutter-apk", "app-"+variant+".apk")
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("build passed, but no APK was found in %s", filepath.Join(root, "build", "app", "outputs", "flutter-apk"))
	}
	outDir := filepath.Join(root, "apk")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create apk folder: %v", err)
	}
	dest := filepath.Join(outDir, filepath.Base(root)+"-"+variant+".apk")
	if err := copyFile(src, dest); err != nil {
		return "", fmt.Errorf("could not copy APK to %s: %v", dest, err)
	}
	return dest, nil
}

func cmdRun(clean, release bool, device string) {
	start := time.Now()
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	if isFlutterProject(root) {
		runFlutter(root, clean, release, device)
		return
	}
	if isLibraryProject(root) {
		die(filepath.Base(root) + " is a library project — nothing to install or launch. Use `skoll build` to make the .aar.")
	}
	task := "installDebug"
	variant := "debug"
	if release {
		task = "installRelease"
		variant = "release"
	}
	say("Project: " + root)
	ensureJava()
	ensureAndroidSdk(root)
	if release {
		ensureReleaseKeystore(root)
	}
	gradle := mustGradle(root)
	say("Gradle: " + gradle)
	ensureAdb()
	serial, err := pickDevice(device)
	if err != nil {
		die(err.Error())
	}
	say("Device: " + serial)
	if clean {
		if err := runLive(gradle, append(gradleArgs(), "clean")...); err != nil {
			die("clean failed: " + err.Error())
		}
	}
	env := []string{"ANDROID_SERIAL=" + serial}
	stepLine("Building and installing " + filepath.Base(root) + " (" + variant + ")…")
	if err := runLiveEnv(gradle, env, append(gradleArgs(), task)...); err != nil {
		die("build/install failed.")
	}
	// Same as `skoll build`: copy the APK to <project>/apk/ too.
	// A release run is checked as well: an unsigned APK cannot have
	// installed (installRelease fails first), but a debug-signed one
	// installs fine — and the user should know they just ran a
	// debug-signed "release".
	dest, err := copyBuiltApk(root, variant)
	if err != nil {
		die(err.Error())
	}
	printApkSummary(dest, start)
	if release {
		if err := verifyApkSigned(dest); err != nil {
			die(err.Error())
		}
	}
	pkg := findPackage(root)
	if pkg == "" {
		die("built and installed, but package name not found — open the app manually.")
	}
	launchApp(serial, pkg)
}

func runFlutter(root string, clean, release bool, device string) {
	ensureJava()
	ensureAndroidSdk(root)
	ensureFlutter()
	if release {
		ensureReleaseKeystore(root)
	}
	say("Project: " + root + " (Flutter)")
	if clean {
		if err := runLive("flutter", "clean"); err != nil {
			die("flutter clean failed: " + err.Error())
		}
	}
	// `flutter run` installs, launches, and stays attached (hot reload
	// with r, quit with q). It picks the device itself unless -d is given.
	args := []string{"run"}
	if release {
		args = append(args, "--release")
	}
	if device != "" {
		args = append(args, "-d", device)
	}
	stepLine("Running on your phone — hot reload with r, quit with q…")
	if err := runLive("flutter", args...); err != nil {
		die("flutter run failed.")
	}
	// Copy the APK it built, like the Gradle path does. No time here:
	// the attached session length is not the build time. A release
	// run is checked as well: `flutter run --release` installs a
	// debug-signed APK without complaint when no release key is
	// configured, and the user should know.
	variant := "debug"
	if release {
		variant = "release"
	}
	if _, err := os.Stat(filepath.Join(root, "build", "app", "outputs", "flutter-apk", "app-"+variant+".apk")); err == nil {
		dest, err := copyFlutterApk(root, variant)
		if err != nil {
			die(err.Error())
		}
		okLine("APK ready: " + dest + apkSizeSuffix(dest))
		if release {
			if err := verifyApkSigned(dest); err != nil {
				die(err.Error())
			}
		}
	}
}

func cmdUninstall() {
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	pkg := findPackage(root)
	if pkg == "" {
		die("package name not found — build once first (skoll build).")
	}
	ensureAdb()
	serial, err := pickDevice("")
	if err != nil {
		die(err.Error())
	}
	say("Uninstalling " + pkg + " from " + serial)
	if err := runLive("adb", "-s", serial, "uninstall", pkg); err != nil {
		die("uninstall failed: " + err.Error())
	}
}

func cmdClean() {
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	if isFlutterProject(root) {
		ensureFlutter()
		if err := runLive("flutter", "clean"); err != nil {
			die("flutter clean failed: " + err.Error())
		}
		return
	}
	ensureJava()
	gradle := mustGradle(root)
	if err := runLive(gradle, append(gradleArgs(), "clean")...); err != nil {
		die("clean failed: " + err.Error())
	}
}

func cmdLogs() {
	ensureAdb()
	serial, err := pickDevice("")
	if err != nil {
		die(err.Error())
	}
	pkg := ""
	if root, err := findProjectRoot(); err == nil {
		pkg = findPackage(root)
	}
	if pkg != "" {
		say("Logcat for " + pkg + " on " + serial + " (Ctrl+C to stop)")
		// pidof may be empty when the app is not running; logcat with
		// --pid= then shows nothing, so fall back to the full logcat.
		pid, _ := runCapture("adb", "-s", serial, "shell", "pidof", pkg)
		pid = strings.TrimSpace(pid)
		if pid != "" {
			_ = runLive("adb", "-s", serial, "logcat", "--pid="+pid)
			return
		}
	}
	say("Logcat on " + serial + " (Ctrl+C to stop)")
	_ = runLive("adb", "-s", serial, "logcat")
}

var _ = filepath.Join
