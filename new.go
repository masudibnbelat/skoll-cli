package main

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// The Compose + Material 3 starter template, embedded in the binary.
// Placeholders in file contents:
//   {{AppName}}           — the label, e.g. "My App"
//   {{AppNameId}}         — letters/digits only, e.g. "MyApp" (theme + project name)
//   {{Package}}           — e.g. com.example.myapp
//   {{MinSdk}}            — e.g. 26
//   {{GradleVersion}}     — latest at creation time, e.g. 9.3.1
//   {{AgpVersion}}        — e.g. 9.1.1
//   {{KotlinVersion}}     — e.g. 2.2.10
//   {{ComposeBomVersion}} — e.g. 2025.09.00
// The Kotlin files live under com/example/app/ in the template; that
// path segment is replaced with the real package path on generation.
// NOTE: Go embed skips dotfiles, so the template file is named
// `gitignore` and is written out as `.gitignore`.
//
//go:embed templates/compose
var composeTemplate embed.FS

// The Flutter starter template, embedded in the binary — the same
// files `flutter create` makes for an Android-only app (Flutter
// 3.50 stable template), with {{AppName}} / {{Package}} /
// {{FlutterName}} placeholders. Gradle/AGP/Kotlin/Dart versions in
// it are Flutter's own pins (see flutterPinnedVersions), because
// Flutter only supports those combos.
//
//go:embed templates/flutter
var flutterTemplate embed.FS

var (
	newPackage string
	newMinSdk  int
	newOutput  string
	newType    string
)

var newCmd = &cobra.Command{
	Use:     "new [name]",
	Aliases: []string{"n"},
	Short:   "Create a new Compose + Material 3 Android project",
	Long: `Creates a new Android project from the built-in template, like
create-next-app does for the web: one command, one ready project.

It asks three things — the project name, the project type, then
the package name (the example is shown; press Enter to accept it):

  Project name: MyApp
  Project type:
    1) Kotlin (Compose + Material 3)
    2) Flutter
  Choose [1]:
  Package name [com.example.myapp]:

Give the name on the command line to skip the first question:
angrboda new MyApp. Give -t flutter for a Flutter project, -p
for your own package, to skip the questions.

A Kotlin project always uses the latest stable versions (Kotlin,
AGP, Gradle, Compose BOM are looked up when you run it; pinned
versions are used when there is no internet). A Flutter project
uses Flutter's own pinned versions instead (Gradle 9.5.0, AGP
9.3.1, Kotlin 2.4.20, Dart ^3.14.0) — Flutter only supports those
combos. Build it with: cd <name> && angrboda build`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cmdNew(args)
	},
}

func init() {
	newCmd.Flags().StringVarP(&newPackage, "package", "p", "", "Application ID / package, e.g. com.masud.moan (default: com.example.<name>)")
	newCmd.Flags().IntVar(&newMinSdk, "min-sdk", 26, "Minimum Android API level")
	newCmd.Flags().StringVar(&newOutput, "output", "", "Output folder (default: ./<name>)")
	newCmd.Flags().StringVarP(&newType, "type", "t", "", "Project type: kotlin or flutter (default: ask)")
}

var packageRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
var nonAlnumRe = regexp.MustCompile(`[^A-Za-z0-9]+`)

// prompt asks one question and returns the answer. An empty answer
// returns "". Reading from a terminal, a pipe, or /dev/null all work:
// /dev/null answers "" at once, so a missing name dies instead of
// hanging.
func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func cmdNew(args []string) {
	reader := bufio.NewReader(os.Stdin)
	name := ""
	if len(args) == 1 {
		name = strings.TrimSpace(args[0])
	}
	// Step 1 — project name.
	if name == "" {
		name = prompt(reader, "Project name: ")
		if name == "" {
			die("give the app a name: angrboda new MyApp")
		}
	}
	// "My App" -> "MyApp": used for the theme, the Gradle project
	// name, and the default folder (no spaces in folder names).
	appID := nonAlnumRe.ReplaceAllString(name, "")
	if appID == "" {
		die("name must contain at least one letter or digit: angrboda new MyApp")
	}
	// Theme and Gradle names must start with a letter.
	if appID[0] >= '0' && appID[0] <= '9' {
		appID = "App" + appID
	}
	// Step 2 — project type: Kotlin or Flutter.
	projType := strings.ToLower(strings.TrimSpace(newType))
	if projType == "" {
		fmt.Println("Project type:")
		fmt.Println("  1) Kotlin (Compose + Material 3)")
		fmt.Println("  2) Flutter")
		switch strings.ToLower(prompt(reader, "Choose [1]: ")) {
		case "", "1", "kotlin", "k":
			projType = "kotlin"
		case "2", "flutter", "f":
			projType = "flutter"
		default:
			die("pick 1 for Kotlin or 2 for Flutter: angrboda new MyApp")
		}
	}
	if projType != "kotlin" && projType != "flutter" {
		die("bad --type: use kotlin or flutter")
	}
	// Step 3 — package name, with the example as the default.
	defaultPkg := "com.example." + strings.ToLower(appID)
	pkg := newPackage
	if pkg == "" {
		pkg = prompt(reader, "Package name ["+defaultPkg+"]: ")
		if pkg == "" {
			pkg = defaultPkg
		}
	}
	if !packageRe.MatchString(pkg) {
		die("bad package '" + pkg + "' — use lower-case letters, digits and dots, e.g. com.example.myapp")
	}
	if newMinSdk < 21 || newMinSdk > 36 {
		die("bad --min-sdk: use 21..36")
	}
	out := newOutput
	if out == "" {
		out = appID
	}
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		die("folder " + out + " already exists and is not empty — pick another name or --output.")
	}
	// Versions + template. Kotlin gets the latest stable versions
	// (pinned fallback when offline), with a spinner while they are
	// looked up. Flutter uses its own pins — no lookup.
	var vers appVersions
	var templateFS embed.FS
	var templateRoot string
	if projType == "flutter" {
		vers = flutterPinnedVersions
		templateFS = flutterTemplate
		templateRoot = "templates/flutter"
		say("Flutter versions: Gradle 9.5.0, AGP 9.3.1, Kotlin 2.4.20, Dart ^3.14.0 (Flutter's own pins)")
	} else {
		sp := startSpinner("Finding latest versions")
		vers = resolveLatestVersions()
		sp.stop(vers.describe())
		templateFS = composeTemplate
		templateRoot = "templates/compose"
	}
	pkgPath := strings.ReplaceAll(pkg, ".", "/")
	replacements := []struct{ old, new string }{
		{"{{AppName}}", name},
		{"{{AppNameId}}", appID},
		{"{{FlutterName}}", strings.ToLower(appID)},
		{"{{Package}}", pkg},
		{"{{MinSdk}}", strconv.Itoa(newMinSdk)},
		{"{{GradleVersion}}", vers.gradle},
		{"{{AgpVersion}}", vers.agp},
		{"{{KotlinVersion}}", vers.kotlin},
		{"{{ComposeBomVersion}}", vers.bom},
	}
	written := 0
	err := fs.WalkDir(templateFS, templateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, templateRoot+"/")
		if rel == "" || rel == "." {
			return nil
		}
		rel = strings.ReplaceAll(rel, "com/example/app", pkgPath)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(out, filepath.FromSlash(rel)), 0o755)
		}
		if filepath.Base(rel) == "gitignore" {
			rel = filepath.Join(filepath.Dir(rel), ".gitignore")
		}
		data, err := templateFS.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, r := range replacements {
			content = strings.ReplaceAll(content, r.old, r.new)
		}
		dest := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			return err
		}
		written++
		return nil
	})
	if err != nil {
		die("could not create project: " + err.Error())
	}
	say(fmt.Sprintf("Created %s in %s (%d files)", name, out, written))
	fmt.Println("Package: " + pkg)
	fmt.Println("Next: cd " + out + " && angrboda build")
	if projType == "flutter" {
		fmt.Println("Note: needs the Flutter SDK — `flutter --version` should work first. Android only; add other platforms later with `flutter create --platforms=ios .`.")
	}
}
