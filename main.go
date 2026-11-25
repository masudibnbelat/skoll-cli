package main

// angrboda — one small CLI for any Android project.
// Go port of the Bash draft in workspace/android-cli/andro.
// Cobra-based CLI: the command tree lives in commands.go
// (run/build/devices/logs/uninstall/clean/doctor), release.go
// (release) and new.go (new); the Android / Gradle / Java logic
// underneath is unchanged. Cobra gives real --help per command,
// flag errors, shell completion, and one-line command definitions
// — adding a new command is one more *cobra.Command.
//
//   CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o angrboda-linux-amd64 .
//   CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o angrboda-windows-amd64.exe .
//
// No project name, package, or Gradle version is hardcoded here.
// Gradle is picked dynamically from the project's own wrapper version.

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.6.0"

func main() {
	ensureAndroidEnv()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "angrboda",
	Short: "One small CLI for any Android project",
	Long: `angrboda — one small CLI for any Android project.

Gradle is never hardcoded: the version comes from the project's
gradle/wrapper/gradle-wrapper.properties (distributionUrl).
./gradlew is used first; if it is missing, the exact wrapper version
is looked up locally, and if it is not there it is downloaded once
to ~/.angrboda/gradle/gradle-<version>/ (Linux) or
%USERPROFILE%\.angrboda\gradle\gradle-<version>\ (Windows).
The Gradle on PATH is never used as a fallback when the wrapper
wants a different version.

Java and adb install themselves when missing: no usable Java 17-21
-> Eclipse Temurin 17 is downloaded once to ~/.angrboda/jdk/
(%USERPROFILE%\.angrboda\jdk\ on Windows); no adb -> Google's
platform-tools zip is downloaded once to ~/.angrboda/platform-tools/.
Building still needs the full Android SDK (platforms + build-tools).`,
	Version: version,
	// Bare `angrboda` prints help, like before.
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the angrboda version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("angrboda " + version)
	},
}

func init() {
	rootCmd.SetVersionTemplate("angrboda {{.Version}}\n")
	rootCmd.AddCommand(
		newCmd,
		runCmd,
		buildCmd,
		releaseCmd,
		devicesCmd,
		logsCmd,
		uninstallCmd,
		cleanCmd,
		doctorCmd,
		versionCmd,
	)
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, "STOP: "+msg)
	os.Exit(1)
}

func say(msg string) {
	fmt.Println("== " + msg)
}
