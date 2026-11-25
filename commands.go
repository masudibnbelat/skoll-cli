package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Flags for `angrboda build` and `angrboda run`, bound in init() below.
var (
	buildRelease bool
	buildClean   bool
	runClean     bool
	runRelease   bool
	runDevice    string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the APK, no install",
	Long: `Build only — never install. assembleDebug needs no phone and no adb.
The APK is copied to <project>/apk/<project>-debug.apk (or -release.apk).
For a Flutter project it runs ` + "`flutter build apk`" + ` instead.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdBuild(buildRelease, buildClean)
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
	fmt.Println("angrboda " + version + " — environment check")
	fmt.Println("ANDROID_HOME: " + os.Getenv("ANDROID_HOME"))
	// Java and adb are reported from where angrboda would actually use
	// them (system first, angrboda cache second). Doctor never downloads
	// anything — the install happens on the first build / run.
	if bin, _, src := findJava(); bin != "" {
		fmt.Println("java: " + bin + " (" + src + ")")
	} else {
		fmt.Println("java: MISSING (auto-installs Temurin 17 on build)")
	}
	if p, src := findAdb(); p != "" {
		fmt.Println("adb: " + p + " (" + src + ")")
	} else {
		fmt.Println("adb: MISSING (auto-installs platform-tools on run)")
	}
	for _, tool := range []string{"gh", "gradle", "flutter"} {
		if p, err := lookPath(tool); err == nil {
			fmt.Println(tool + ": " + p)
		} else {
			fmt.Println(tool + ": MISSING")
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
		return
	}
	fmt.Println("Project type: Kotlin (Gradle)")
	if v := wrapperGradleVersion(root); v != "" {
		fmt.Println("Wrapper Gradle version: " + v)
	} else {
		fmt.Println("Wrapper Gradle version: not found in gradle-wrapper.properties")
	}
	if g, err := pickGradle(root); err == nil {
		fmt.Println("Gradle: " + g)
	} else {
		fmt.Println("Gradle: none usable (" + err.Error() + ")")
	}
}

func mustCwd() string {
	d, _ := os.Getwd()
	return d
}

func cmdDevices() {
	ensureAdb()
	if err := runLive("adb", "devices"); err != nil {
		die("adb devices failed: " + err.Error())
	}
}

func cmdBuild(release, clean bool) {
	// Build only — never install. assembleDebug needs no phone and no adb.
	// (installDebug would fail with "No connected devices!" when no phone
	// is plugged in, so it belongs to `angrboda run` only, not to `angrboda build`.)
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	if isFlutterProject(root) {
		buildFlutter(root, release, clean)
		return
	}
	task := "assembleDebug"
	if release {
		task = "assembleRelease"
	}
	say("Project: " + root)
	ensureJava()
	gradle := mustGradle(root)
	say("Gradle: " + gradle)
	all := append(gradleArgs(), task)
	if clean {
		if err := runLive(gradle, append(gradleArgs(), "clean")...); err != nil {
			die("clean failed: " + err.Error())
		}
	}
	if err := runLive(gradle, all...); err != nil {
		die("build failed: " + err.Error())
	}
	// Copy the built APK to <project>/apk/<project>-debug.apk (or
	// -release.apk), so it is easy to find and share — no digging
	// through app/build/outputs/apk/. Copy, not move: the original
	// stays where Gradle put it.
	variant := "debug"
	if release {
		variant = "release"
	}
	copyBuiltApk(root, variant)
}

// copyBuiltApk finds the APK Gradle just built (app/build/outputs/apk/
// <variant>/*.apk), creates <root>/apk/ if needed, and copies the APK
// there renamed to <project>-<variant>.apk, e.g. MyApp-debug.apk.
// The original file stays in place. An existing file with the same
// name is replaced.
func copyBuiltApk(root, variant string) {
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
		die("build passed, but no APK was found in " + apkDir)
	}
	outDir := filepath.Join(root, "apk")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		die("could not create apk folder: " + err.Error())
	}
	dest := filepath.Join(outDir, filepath.Base(root)+"-"+variant+".apk")
	if err := copyFile(src, dest); err != nil {
		die("could not copy APK to " + dest + ": " + err.Error())
	}
	say("APK: " + dest)
}

// buildFlutter builds a Flutter project's APK with the Flutter SDK
// and copies it to <project>/apk/<project>-<variant>.apk, like the
// Gradle path does for Kotlin projects.
func buildFlutter(root string, release, clean bool) {
	if _, err := lookPath("flutter"); err != nil {
		die("Flutter SDK not found on PATH. Install Flutter (https://docs.flutter.dev/get-started/install) and retry — `flutter --version` should work first.")
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
	if err := runLive("flutter", "build", "apk", mode); err != nil {
		die("flutter build failed: " + err.Error())
	}
	copyFlutterApk(root, variant)
}

// copyFlutterApk copies the APK `flutter build apk` just made
// (build/app/outputs/flutter-apk/app-<variant>.apk) to
// <project>/apk/<project>-<variant>.apk. Copy, not move: the
// original stays where Flutter put it.
func copyFlutterApk(root, variant string) {
	src := filepath.Join(root, "build", "app", "outputs", "flutter-apk", "app-"+variant+".apk")
	if _, err := os.Stat(src); err != nil {
		die("build passed, but no APK was found in " + filepath.Join(root, "build", "app", "outputs", "flutter-apk"))
	}
	outDir := filepath.Join(root, "apk")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		die("could not create apk folder: " + err.Error())
	}
	dest := filepath.Join(outDir, filepath.Base(root)+"-"+variant+".apk")
	if err := copyFile(src, dest); err != nil {
		die("could not copy APK to " + dest + ": " + err.Error())
	}
	say("APK: " + dest)
}

func cmdRun(clean, release bool, device string) {
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	if isFlutterProject(root) {
		runFlutter(root, clean, release, device)
		return
	}
	task := "installDebug"
	if release {
		task = "installRelease"
	}
	say("Project: " + root)
	ensureJava()
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
	if err := runLiveEnv(gradle, env, append(gradleArgs(), task)...); err != nil {
		die("build/install failed.")
	}
	// Same as `angrboda build`: copy the APK to <project>/apk/ too.
	variant := "debug"
	if release {
		variant = "release"
	}
	copyBuiltApk(root, variant)
	pkg := findPackage(root)
	if pkg == "" {
		die("built and installed, but package name not found — open the app manually.")
	}
	launchApp(serial, pkg)
}

func runFlutter(root string, clean, release bool, device string) {
	if _, err := lookPath("flutter"); err != nil {
		die("Flutter SDK not found on PATH. Install Flutter (https://docs.flutter.dev/get-started/install) and retry — `flutter --version` should work first.")
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
	if err := runLive("flutter", args...); err != nil {
		die("flutter run failed.")
	}
	// Copy the APK it built, like the Gradle path does.
	variant := "debug"
	if release {
		variant = "release"
	}
	if _, err := os.Stat(filepath.Join(root, "build", "app", "outputs", "flutter-apk", "app-"+variant+".apk")); err == nil {
		copyFlutterApk(root, variant)
	}
}

func cmdUninstall() {
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	pkg := findPackage(root)
	if pkg == "" {
		die("package name not found — build once first (angrboda build).")
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
		if _, err := lookPath("flutter"); err != nil {
			die("Flutter SDK not found on PATH. Install Flutter (https://docs.flutter.dev/get-started/install) and retry — `flutter --version` should work first.")
		}
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
