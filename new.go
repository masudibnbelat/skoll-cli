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
//
//	{{AppName}}           — the label, e.g. "My App"
//	{{AppNameId}}         — letters/digits only, e.g. "MyApp" (theme + project name)
//	{{Package}}           — e.g. com.example.myapp
//	{{MinSdk}}            — e.g. 26
//	{{GradleVersion}}     — latest at creation time, e.g. 9.3.1
//	{{AgpVersion}}        — e.g. 9.1.1
//	{{KotlinVersion}}     — e.g. 2.2.10
//	{{ComposeBomVersion}} — e.g. 2025.09.00
//
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

// The Empty starter template: Kotlin, no Compose, one Activity with
// a "Hello, <name>!" TextView. Same Gradle/AGP/Kotlin versions as
// the Compose template, same release signing block.
//
//go:embed templates/empty
var emptyTemplate embed.FS

// The Library starter template: one Android library module, no app.
// Builds an .aar with `angrboda build`.
//
//go:embed templates/library
var libraryTemplate embed.FS

var (
	newPackage    string
	newMinSdk     int
	newOutput     string
	newType       string
	newGradlePin  string
	newAgpPin     string
	newKotlinPin  string
	newBomPin     string
)

var newCmd = &cobra.Command{
	Use:     "new [name]",
	Aliases: []string{"n"},
	Short:   "Create a new Android project",
	Long: `Creates a new Android project from the built-in template, like
create-next-app does for the web: one command, one ready project.

It asks three things — the project name, the project type, then
the package name (the example is shown; press Enter to accept it):

  Project name: MyApp
  Project type:
    1) Kotlin (Compose + Material 3)
    2) Flutter
    3) Empty (Kotlin, no Compose)
    4) Library (Android .aar)
  Choose [1]:
  Package name [com.example.myapp]:

Give the name on the command line to skip the first question:
angrboda new MyApp. Give -t flutter for a Flutter project, -p
for your own package, to skip the questions. To create a project
in the current folder instead, use ` + "`angrboda init`" + `.

A Kotlin project always uses the latest stable versions (Kotlin,
AGP, Gradle, Compose BOM are looked up when you run it; pinned
versions are used when there is no internet). Pin one with
--gradle, --agp, --kotlin or --bom. A Flutter project uses
Flutter's own pinned versions instead (Gradle 9.5.0, AGP 9.3.1,
Kotlin 2.4.20, Dart ^3.14.0) — Flutter only supports those combos.
Build it with: cd <name> && angrboda build`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cmdNew(args)
	},
}

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a project in this folder",
	Long: `Creates a new Android project in the current folder, instead
of a new subfolder like ` + "`angrboda new`" + ` does. The project
name defaults to the folder's name; the type and package questions
are the same as ` + "`angrboda new`" + `.

The folder must be empty (a .git folder is fine) — init never
writes over existing files.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cmdInit(args)
	},
}

func init() {
	newCmd.Flags().StringVarP(&newPackage, "package", "p", "", "Application ID / package, e.g. com.masud.moan (default: com.example.<name>)")
	newCmd.Flags().IntVar(&newMinSdk, "min-sdk", 26, "Minimum Android API level")
	newCmd.Flags().StringVar(&newOutput, "output", "", "Output folder (default: ./<name>)")
	newCmd.Flags().StringVarP(&newType, "type", "t", "", "Project type: kotlin, flutter, empty or library (default: ask)")
	newCmd.Flags().StringVar(&newGradlePin, "gradle", "", "Pin the Gradle version, e.g. 9.3.1 (default: latest stable)")
	newCmd.Flags().StringVar(&newAgpPin, "agp", "", "Pin the Android Gradle Plugin version, e.g. 9.1.1 (default: latest stable)")
	newCmd.Flags().StringVar(&newKotlinPin, "kotlin", "", "Pin the Kotlin version, e.g. 2.2.10 (default: latest stable)")
	newCmd.Flags().StringVar(&newBomPin, "bom", "", "Pin the Compose BOM version, e.g. 2025.09.00 (default: latest stable)")
	initCmd.Flags().StringVarP(&newPackage, "package", "p", "", "Application ID / package, e.g. com.masud.moan (default: com.example.<name>)")
	initCmd.Flags().IntVar(&newMinSdk, "min-sdk", 26, "Minimum Android API level")
	initCmd.Flags().StringVarP(&newType, "type", "t", "", "Project type: kotlin, flutter, empty or library (default: ask)")
	initCmd.Flags().StringVar(&newGradlePin, "gradle", "", "Pin the Gradle version, e.g. 9.3.1 (default: latest stable)")
	initCmd.Flags().StringVar(&newAgpPin, "agp", "", "Pin the Android Gradle Plugin version, e.g. 9.1.1 (default: latest stable)")
	initCmd.Flags().StringVar(&newKotlinPin, "kotlin", "", "Pin the Kotlin version, e.g. 2.2.10 (default: latest stable)")
	initCmd.Flags().StringVar(&newBomPin, "bom", "", "Pin the Compose BOM version, e.g. 2025.09.00 (default: latest stable)")
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

// appIDFromName turns "My App" into "MyApp": the theme, Gradle
// project name and default folder (no spaces in folder names).
// Theme and Gradle names must start with a letter.
func appIDFromName(name string) string {
	appID := nonAlnumRe.ReplaceAllString(name, "")
	if appID == "" {
		die("name must contain at least one letter or digit: angrboda new MyApp")
	}
	if appID[0] >= '0' && appID[0] <= '9' {
		appID = "App" + appID
	}
	return appID
}

// askProjectType resolves the project type: the -t flag wins,
// otherwise it asks. "lib" is accepted for "library".
func askProjectType(reader *bufio.Reader) string {
	projType := strings.ToLower(strings.TrimSpace(newType))
	if projType == "" {
		fmt.Println("Project type:")
		fmt.Println("  1) Kotlin (Compose + Material 3)")
		fmt.Println("  2) Flutter")
		fmt.Println("  3) Empty (Kotlin, no Compose)")
		fmt.Println("  4) Library (Android .aar)")
		switch strings.ToLower(prompt(reader, "Choose [1]: ")) {
		case "", "1", "kotlin", "k":
			projType = "kotlin"
		case "2", "flutter", "f":
			projType = "flutter"
		case "3", "empty", "e":
			projType = "empty"
		case "4", "library", "lib", "l":
			projType = "library"
		default:
			die("pick 1 for Kotlin, 2 for Flutter, 3 for Empty or 4 for Library: angrboda new MyApp")
		}
	}
	if projType == "lib" {
		projType = "library"
	}
	switch projType {
	case "kotlin", "flutter", "empty", "library":
	default:
		die("bad --type: use kotlin, flutter, empty or library")
	}
	return projType
}

// askPackage resolves the package name: the -p flag wins, otherwise
// it asks with com.example.<name> as the default.
func askPackage(reader *bufio.Reader, appID string) string {
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
	return pkg
}

// resolveProjectVersions picks the versions and the template for a
// project type. Kotlin, Empty and Library get the latest stable
// versions (pinned fallback when offline, --gradle/--agp/--kotlin/
// --bom overrides when given). Flutter uses its own pins — version
// pins are ignored there, with a warning.
func resolveProjectVersions(projType string) (appVersions, embed.FS, string) {
	switch projType {
	case "flutter":
		if anyPinSet() {
			warnLine("version pins are ignored for Flutter — it uses Flutter's own pinned versions (Gradle 9.5.0, AGP 9.3.1, Kotlin 2.4.20)")
		}
		say("Flutter versions: Gradle 9.5.0, AGP 9.3.1, Kotlin 2.4.20, Dart ^3.14.0 (Flutter's own pins)")
		return flutterPinnedVersions, flutterTemplate, "templates/flutter"
	case "empty":
		sp := startSpinner("Finding latest versions")
		vers := applyVersionPins(resolveLatestVersions())
		sp.stop(vers.describeWithoutBom())
		return vers, emptyTemplate, "templates/empty"
	case "library":
		sp := startSpinner("Finding latest versions")
		vers := applyVersionPins(resolveLatestVersions())
		sp.stop(vers.describeWithoutBom())
		return vers, libraryTemplate, "templates/library"
	default: // kotlin
		sp := startSpinner("Finding latest versions")
		vers := applyVersionPins(resolveLatestVersions())
		sp.stop(vers.describe())
		return vers, composeTemplate, "templates/compose"
	}
}

func replacementsFor(name, appID, pkg string, vers appVersions) []struct{ old, new string } {
	return []struct{ old, new string }{
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
}

// writeTemplate writes the template files into out, applying the
// replacements. "com/example/app" path segments become the real
// package path, and a file named `gitignore` becomes `.gitignore`.
// It returns the number of files written.
func writeTemplate(templateFS embed.FS, templateRoot string, replacements []struct{ old, new string }, out string) int {
	pkgPath := ""
	for _, r := range replacements {
		if r.old == "{{Package}}" {
			pkgPath = strings.ReplaceAll(r.new, ".", "/")
		}
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
	return written
}

func printCreated(name, out, projType, pkg string, written int) {
	okLine(fmt.Sprintf("Created %s in %s (%d files)", name, out, written))
	fmt.Println("Package: " + pkg)
	next := "cd " + out + " && angrboda build"
	if out == "." {
		next = "angrboda build"
	}
	switch projType {
	case "library":
		fmt.Println("Library project — `angrboda build` makes the .aar; there is no app to run.")
		fmt.Println("Next: " + next)
	case "flutter":
		fmt.Println("Next: " + next)
		fmt.Println("Note: needs the Flutter SDK — `flutter --version` should work first. Android only; add other platforms later with `flutter create --platforms=ios .`.")
	default:
		fmt.Println("Next: " + next)
	}
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
	appID := appIDFromName(name)
	// Step 2 — project type.
	projType := askProjectType(reader)
	// Step 3 — package name, with the example as the default.
	pkg := askPackage(reader, appID)
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
	vers, templateFS, templateRoot := resolveProjectVersions(projType)
	written := writeTemplate(templateFS, templateRoot, replacementsFor(name, appID, pkg, vers), out)
	printCreated(name, out, projType, pkg, written)
}

func cmdInit(args []string) {
	cwd, _ := os.Getwd()
	name := filepath.Base(cwd)
	if len(args) == 1 {
		name = strings.TrimSpace(args[0])
	}
	if name == "" || name == "/" || name == "." {
		die("could not use this folder's name — give one: angrboda init MyApp")
	}
	appID := appIDFromName(name)
	// The folder must be empty (a .git folder is fine) — init never
	// writes over existing files.
	if entries, err := os.ReadDir("."); err == nil {
		for _, e := range entries {
			if e.Name() != ".git" {
				die("this folder is not empty — init needs an empty folder (a .git folder is fine).")
			}
		}
	}
	reader := bufio.NewReader(os.Stdin)
	projType := askProjectType(reader)
	pkg := askPackage(reader, appID)
	if newMinSdk < 21 || newMinSdk > 36 {
		die("bad --min-sdk: use 21..36")
	}
	vers, templateFS, templateRoot := resolveProjectVersions(projType)
	written := writeTemplate(templateFS, templateRoot, replacementsFor(name, appID, pkg, vers), ".")
	printCreated(name, ".", projType, pkg, written)
}
