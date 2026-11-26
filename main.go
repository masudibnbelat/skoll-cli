// Location: main.go
package main

// skoll — one small CLI for any Android project.
// Go port of the Bash draft in workspace/android-cli/andro.
// Cobra-based CLI: the command tree lives in commands.go
// (run/build/devices/logs/uninstall/clean/doctor), release.go
// (release) and new.go (new); the Android / Gradle / Java logic
// underneath is unchanged. Cobra gives real --help per command,
// flag errors, shell completion, and one-line command definitions
// — adding a new command is one more *cobra.Command.
//
//   CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o skoll-linux-amd64 .
//   CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o skoll-windows-amd64.exe .
//
// No project name, package, or Gradle version is hardcoded here.
// Gradle is picked dynamically from the project's own wrapper version.

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "1.0.5"

func main() {
	cleanStaleOldBinary()
	ensureAndroidEnv()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// cleanStaleOldBinary removes the .old file a Windows self-update
// leaves behind (see replaceCurrentBinary in update.go). On Unix
// there is never one.
func cleanStaleOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe + ".old")
}

var rootCmd = &cobra.Command{
	Use:   "skoll",
	Short: "One small CLI for any Android project",
	Long: `skoll — one small CLI for any Android project.

Gradle is never hardcoded: the version comes from the project's
gradle/wrapper/gradle-wrapper.properties (distributionUrl).
./gradlew is used first; if it is missing, the exact wrapper version
is looked up locally, and if it is not there it is downloaded once
to ~/.skoll/gradle/gradle-<version>/ (Linux) or
%USERPROFILE%\.skoll\gradle\gradle-<version>\ (Windows).
The Gradle on PATH is never used as a fallback when the wrapper
wants a different version.

Java, adb, the Android SDK and Flutter install themselves when missing:
no usable Java 17-21 -> Eclipse Temurin 17 is downloaded once to ~/.skoll/jdk/
(%USERPROFILE%\.skoll\jdk\ on Windows); no adb -> Google's
platform-tools zip is downloaded once to ~/.skoll/platform-tools/;
no Android SDK -> cmdline-tools + platform-tools + the platform and
build-tools for the project's API level are installed once to ~/Android/Sdk
(%LOCALAPPDATA%\Android\Sdk on Windows); no flutter -> the latest
stable Flutter SDK is downloaded once to ~/.skoll/flutter/.
Release builds are signed: the first one creates a release keystore
once at ~/.skoll/keystore/ (see skoll keystore) — back it up.
skoll update updates skoll itself to the latest release.`,
	Version: version,
	// Bare `skoll` prints help, like before.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		maybeNotifyUpdate(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the skoll version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("skoll " + version)
	},
}

func init() {
	rootCmd.SetVersionTemplate("skoll {{.Version}}\n")
	rootCmd.AddCommand(
		newCmd,
		initCmd,
		runCmd,
		buildCmd,
		releaseCmd,
		keystoreCmd,
		devicesCmd,
		logsCmd,
		uninstallCmd,
		cleanCmd,
		doctorCmd,
		updateCmd,
		versionCmd,
	)
}

func die(msg string) {
	if colorOn(os.Stderr) {
		fmt.Fprintln(os.Stderr, red("STOP: ")+msg)
	} else {
		fmt.Fprintln(os.Stderr, "STOP: "+msg)
	}
	os.Exit(1)
}

func say(msg string) {
	fmt.Println("== " + msg)
}
