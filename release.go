package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// Flags for `skoll release`, bound in init() below.
var (
	releaseMajor  bool
	releaseMinor  bool
	releaseDryRun bool
	releaseYes    bool
	releaseBundle bool
)

var releaseCmd = &cobra.Command{
	Use:   "release [v1.5.0] [\"notes\"]",
	Short: "Tag, build and publish a GitHub release",
	Long: `Tags the next version (or the one you give), builds the release
artifact, checks it, pushes code + tag, and creates the GitHub
release with the artifact attached.

The version is bumped for you: Kotlin — versionName in
app/build.gradle.kts becomes the tag and versionCode goes up by
one; Flutter — version in pubspec.yaml becomes the tag and the
build number goes up by one; library — the tag is the version,
there is nothing in the code to bump. The bump is computed from
the newer of the latest git tag and the version the code already
declares, so a fresh template (version 1.0, no tags yet) releases
as v1.0.1, not v0.0.1.

A dirty tree stops the release unless --yes is given. If the build
or the checks fail, the local tag is removed again so you can
retry. The release APK is signed (see skoll keystore),
and its versionName stamp and signature are checked before upload.
With --bundle the App Bundle (.aab) for Google Play is built and
uploaded too — new apps on Google Play need an .aab, not an .apk.`,
	Args: cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cmdRelease(args)
	},
}

func init() {
	releaseCmd.Flags().BoolVar(&releaseMajor, "major", false, "Bump the major version (v1.2.3 -> v2.0.0)")
	releaseCmd.Flags().BoolVar(&releaseMinor, "minor", false, "Bump the minor version (v1.2.3 -> v1.3.0)")
	releaseCmd.Flags().BoolVar(&releaseDryRun, "dry-run", false, "Show the tag and stop before commit / tag / build / push")
	releaseCmd.Flags().BoolVar(&releaseYes, "yes", false, "Release with uncommitted changes (a dirty tree stops otherwise)")
	releaseCmd.Flags().BoolVar(&releaseBundle, "bundle", false, "Also build and upload the App Bundle (.aab) for Google Play")
}

// cmdRelease mirrors the Bash release helper, with the same three fixes:
//   - no `git add -A` auto-commit on a dirty tree unless --yes,
//   - the tag is removed again if the build / stamp check fails,
//   - the APK versionName stamp must match the tag before upload.
//
// The tag and notes still come from the positional args; the flags
// (--major/--minor/--dry-run/--yes) are Cobra flags now.
func cmdRelease(args []string) {
	var raw, notes string
	bump := ""
	if releaseMajor {
		bump = "major"
	}
	if releaseMinor {
		bump = "minor"
	}
	dryRun, yes := releaseDryRun, releaseYes
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "v") || (len(a) > 0 && a[0] >= '0' && a[0] <= '9'):
			if raw == "" {
				raw = a
			} else if notes == "" {
				notes = a
			} else {
				die("unexpected argument: " + a)
			}
		default:
			if notes == "" {
				notes = a
			} else {
				die("unexpected argument: " + a)
			}
		}
	}

	if _, err := runCapture("git", "rev-parse", "--is-inside-work-tree"); err != nil {
		die("not inside a git project.")
	}
	root := mustRoot()
	if err := os.Chdir(root); err != nil {
		die(err.Error())
	}
	say("Project: " + root)
	kind := projectKind(root)
	if releaseBundle && kind == "library" {
		// Checked here, before any tag exists: a library has no
		// App Bundle, so this must fail before the tag, not after.
		die("a library project has no App Bundle — drop --bundle.")
	}

	if err := runLive("git", "fetch", "--tags", "origin"); err != nil {
		if dryRun {
			fmt.Println("WARNING: git fetch failed (no origin?) — continuing dry-run only.")
		} else {
			die("git fetch failed.")
		}
	}
	latest, _ := runCapture("git", "describe", "--tags", "--abbrev=0")
	latest = strings.TrimSpace(latest)
	if latest == "" {
		latest = "v0.0.0"
	}
	// The version the code declares can be ahead of the tags (a fresh
	// template says 1.0 with no tags yet) — bump from the newer of the two.
	base := latest
	if v := codeVersion(root, kind); v != "" && compareVersions(v, strings.TrimPrefix(latest, "v")) > 0 {
		base = "v" + v
	}

	tag := ""
	if raw == "" {
		ver := strings.TrimPrefix(base, "v")
		parts := strings.Split(ver, ".")
		major, minor, patch := 0, 0, 0
		if len(parts) > 0 {
			major, _ = strconv.Atoi(parts[0])
		}
		if len(parts) > 1 {
			minor, _ = strconv.Atoi(parts[1])
		}
		if len(parts) > 2 {
			patch, _ = strconv.Atoi(parts[2])
		}
		switch bump {
		case "major":
			tag = fmt.Sprintf("v%d.0.0", major+1)
		case "minor":
			tag = fmt.Sprintf("v%d.%d.0", major, minor+1)
		default:
			tag = fmt.Sprintf("v%d.%d.%d", major, minor, patch+1)
		}
	} else if strings.HasPrefix(raw, "v") {
		tag = raw
	} else {
		tag = "v" + raw
	}
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(tag) {
		die("bad version '" + tag + "' — use a form like v1.4.3")
	}
	if _, err := runCapture("git", "rev-parse", tag); err == nil {
		die("tag " + tag + " already exists locally. Use a fresh number.")
	}
	if out, _ := runCapture("git", "ls-remote", "--tags", "origin", "refs/tags/"+tag); strings.TrimSpace(out) != "" {
		die("tag " + tag + " already exists on GitHub. Use a fresh number.")
	}
	if !isNewer(tag, base) {
		die(tag + " is not newer than " + base + ".")
	}

	// Dirty tree stops unless --yes, and the status is shown.
	if out, _ := runCapture("git", "status", "--porcelain"); strings.TrimSpace(out) != "" {
		fmt.Println("Working tree is not clean:")
		if s, _ := runCapture("git", "status", "--short"); s != "" {
			fmt.Print(s)
		}
		if !yes {
			die("commit or stash first, or re-run with --yes to release with these changes.")
		}
	}
	branch, _ := runCapture("git", "branch", "--show-current")
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	say(base + " -> " + tag + " (branch " + branch + ")")
	// Say what the version bump will do, before anything changes.
	switch kind {
	case "kotlin":
		if v := codeVersion(root, kind); v != "" {
			fmt.Println("Would set versionName " + v + " -> " + strings.TrimPrefix(tag, "v") + " in app/build.gradle.kts")
		}
	case "flutter":
		if ver, build, ok := parsePubspecVersion(root); ok {
			fmt.Println("Would set pubspec version " + ver + "+" + strconv.Itoa(build) + " -> " + strings.TrimPrefix(tag, "v") + "+" + strconv.Itoa(build+1))
		}
	}
	if dryRun {
		fmt.Println("DRY RUN: stopping before commit / tag / build / push.")
		return
	}

	// Java is needed for the build below; install it before the tag
	// is created, so a failed download leaves no stale tag behind.
	ensureJava()
	// The Gradle binary is picked here too (no side effects): if it
	// cannot be found, the release stops before any tag exists.
	var gradle string
	if kind != "flutter" {
		var err error
		gradle, err = pickGradle(root)
		if err != nil {
			die(err.Error())
		}
	}
	if kind == "flutter" {
		// Flutter itself is needed for the build; like Java, it is
		// installed before the tag, and it writes nothing into the
		// project.
		ensureFlutter()
	}

	// gh is only needed from here on, so --dry-run works without a login.
	if _, err := lookPath("gh"); err != nil {
		die("GitHub CLI missing. Install once: sudo pacman -S github-cli   then: gh auth login")
	}
	if err := runLive("gh", "auth", "status"); err != nil {
		die("gh is not logged in. Run once: gh auth login")
	}
	if _, err := runCapture("gh", "release", "view", tag); err == nil {
		die("a GitHub release " + tag + " already exists.")
	}

	// The version bump is part of the release commit, so it happens
	// before `git add -A` below.
	switch kind {
	case "kotlin":
		bumpKotlinVersion(root, tag)
	case "flutter":
		bumpPubspecVersion(root, tag)
	}

	if err := runLive("git", "add", "-A"); err != nil {
		die("git add failed: " + err.Error())
	}
	// Empty commit when nothing was staged, otherwise a normal commit.
	if out, _ := runCapture("git", "diff", "--cached", "--quiet"); out == "" {
		// `git diff --cached --quiet` exits 0 when there is nothing staged,
		// but runCapture only returns stdout, so check the exit code directly.
		if exec.Command("git", "diff", "--cached", "--quiet").Run() == nil {
			if err := runLive("git", "commit", "--allow-empty", "-m", tag); err != nil {
				die("git commit failed: " + err.Error())
			}
		} else if err := runLive("git", "commit", "-m", tag); err != nil {
			die("git commit failed: " + err.Error())
		}
	}
	// The release keystore is set up after the commit but before the
	// tag: it writes local.properties / android/key.properties, which
	// must not be part of the release commit, and a failure here
	// leaves no tag behind to clean up.
	if kind != "library" {
		ensureReleaseKeystore(root)
	}

	if err := runLive("git", "tag", tag); err != nil {
		die("git tag failed: " + err.Error())
	}

	// From here until the checks pass, a failed build / stamp /
	// signature check must remove the local tag again, so the
	// release can be retried with the same version. die() calls
	// os.Exit, so a defer CANNOT do this — deferred functions never
	// run on os.Exit — and the cleanup has to happen here, before
	// die(), on every failure path. (That is why the old defer was
	// a bug: a failed release left the tag behind and the retry died
	// with "tag already exists".)
	fail := func(msg string) {
		fmt.Println("Release failed — removing local tag " + tag + " so you can retry.")
		removeLocalTag(tag)
		die(msg)
	}

	describe, _ := runCapture("git", "describe", "--tags", "--abbrev=0")
	if strings.TrimSpace(describe) != tag {
		fail("git describe does not print " + tag + " — NOT building, the artifact stamp would be wrong.")
	}
	say("Tag check OK: " + strings.TrimSpace(describe))

	// Build the artifact for this project kind.
	repo := filepath.Base(root)
	home, _ := os.UserHomeDir()
	var built string // the APK / AAR Gradle (or Flutter) just made
	switch kind {
	case "kotlin":
		say("Gradle: " + gradle)
		if err := runLive(gradle, append(gradleArgs(), "assembleRelease")...); err != nil {
			fail("build failed — nothing was pushed or released.")
		}
		built = findReleaseApk(root)
		if built == "" {
			fail("release APK was not produced.")
		}
		if err := checkApkStamp(built, tag); err != nil {
			fail(err.Error())
		}
		if err := verifyApkSigned(built); err != nil {
			fail(err.Error())
		}
	case "flutter":
		if err := runLive("flutter", "build", "apk", "--release"); err != nil {
			fail("flutter build failed — nothing was pushed or released.")
		}
		built = filepath.Join(root, "build", "app", "outputs", "flutter-apk", "app-release.apk")
		if _, err := os.Stat(built); err != nil {
			fail("release APK was not produced.")
		}
		if err := checkApkStamp(built, tag); err != nil {
			fail(err.Error())
		}
		if err := verifyApkSigned(built); err != nil {
			fail(err.Error())
		}
	default: // library
		say("Gradle: " + gradle)
		if err := runLive(gradle, append(gradleArgs(), "assembleRelease")...); err != nil {
			fail("build failed — nothing was pushed or released.")
		}
		built = findReleaseAar(root)
		if built == "" {
			fail("release .aar was not produced.")
		}
		fmt.Println("Library — no version stamp to check; the tag is the version.")
	}

	// Build + checks passed, but the tag is only safe to keep once
	// the copy and the push succeed too — a failure in either still
	// removes it (see fail() above), so the release can be retried.
	// (The version-bump commit stays even when a release fails and
	// the tag is removed, so a retried release bumps from the bumped
	// code version — a failed release still skips that version
	// number. That is the price of the bump being part of the commit;
	// undoing the commit would be wrong when --yes released a dirty
	// tree with the user's other changes in it.)

	// Same as `skoll build`: copy the artifact to <project>/apk/
	// (or /aar) too (copy, not move — the original stays where the
	// build put it), and to ~/<repo>-<tag>.apk/.aar for upload.
	// With --bundle the App Bundle is built and uploaded too (the
	// APK checks above already proved the signing config, and the
	// .aab is signed with the same one).
	artifacts := []string{}
	var artifact string
	switch kind {
	case "library":
		if _, err := copyBuiltAar(root, "release"); err != nil {
			fail(err.Error())
		}
		artifact = filepath.Join(home, repo+"-"+tag+".aar")
	case "flutter":
		if _, err := copyFlutterApk(root, "release"); err != nil {
			fail(err.Error())
		}
		artifact = filepath.Join(home, repo+"-"+tag+".apk")
	default:
		if _, err := copyBuiltApk(root, "release"); err != nil {
			fail(err.Error())
		}
		artifact = filepath.Join(home, repo+"-"+tag+".apk")
	}
	if err := copyFile(built, artifact); err != nil {
		fail("copy artifact failed: " + err.Error())
	}
	artifacts = append(artifacts, artifact)
	if releaseBundle {
		var bundleBuilt string
		if kind == "flutter" {
			if err := runLive("flutter", "build", "appbundle", "--release"); err != nil {
				fail("flutter appbundle build failed — nothing was pushed or released.")
			}
			bundleBuilt = filepath.Join(root, "build", "app", "outputs", "bundle", "release", "app-release.aab")
			if _, err := os.Stat(bundleBuilt); err != nil {
				fail("release .aab was not produced.")
			}
			if _, err := copyFlutterBundle(root, "release"); err != nil {
				fail(err.Error())
			}
		} else {
			if err := runLive(gradle, append(gradleArgs(), "bundleRelease")...); err != nil {
				fail("bundle build failed — nothing was pushed or released.")
			}
			bundleBuilt = findReleaseBundle(root)
			if bundleBuilt == "" {
				fail("release .aab was not produced.")
			}
			if _, err := copyBuiltBundle(root, "release"); err != nil {
				fail(err.Error())
			}
		}
		bundleArtifact := filepath.Join(home, repo+"-"+tag+".aab")
		if err := copyFile(bundleBuilt, bundleArtifact); err != nil {
			fail("copy bundle failed: " + err.Error())
		}
		artifacts = append(artifacts, bundleArtifact)
	}

	publishRelease(tag, artifacts, notes, repo, branch, kind, fail)
}

// findReleaseBundle finds the release .aab Gradle just built
// (app/build/outputs/bundle/release/app-release.aab, else any .aab
// there). It returns "" when there is none.
func findReleaseBundle(root string) string {
	bundleDir := filepath.Join(root, "app", "build", "outputs", "bundle", "release")
	std := filepath.Join(bundleDir, "app-release.aab")
	if _, err := os.Stat(std); err == nil {
		return std
	}
	entries, _ := os.ReadDir(bundleDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".aab") {
			return filepath.Join(bundleDir, e.Name())
		}
	}
	return ""
}

// publishRelease pushes code + tag and creates the GitHub release
// with the artifacts attached. A failed push removes the local tag
// again (via fail), so the release can be retried; a failed `gh
// release create` does NOT — by then the code and the tag are
// already pushed, and the tag must stay.
func publishRelease(tag string, artifacts []string, notes, repo, branch, kind string, fail func(string)) {
	if err := runLive("git", "push", "origin", branch); err != nil {
		fail("git push branch failed: " + err.Error())
	}
	if err := runLive("git", "push", "origin", tag); err != nil {
		fail("git push tag failed: " + err.Error())
	}
	note := notes
	if note == "" {
		note = "Release " + tag
	}
	args := append([]string{"release", "create", tag}, artifacts...)
	args = append(args, "--title", repo+" "+tag, "--notes", note)
	if err := runLive("gh", args...); err != nil {
		die("gh release create failed — code and tag are pushed, the artifacts are at " + strings.Join(artifacts, ", "))
	}
	latestAPI, _ := runCapture("gh", "api", "repos/{owner}/{repo}/releases/latest", "--jq", ".tag_name")
	fmt.Println("GitHub releases/latest = " + strings.TrimSpace(latestAPI))
	if strings.TrimSpace(latestAPI) != tag {
		fmt.Println("WARNING: releases/latest is not " + tag + " yet — check the release is Published, not Draft/Pre-release.")
	}
	fmt.Println("DONE: " + repo + " " + tag + " is LIVE -> " + strings.Join(artifacts, ", "))
	if kind != "library" {
		fmt.Println("Phones on an older version will be offered this update once; after installing, the app reports v" + strings.TrimPrefix(tag, "v") + " and the updater will say 'You're up to date'.")
	}
}

// codeVersion returns the version the code itself declares, or "":
// Kotlin — versionName in app/build.gradle.kts; Flutter — the version
// in pubspec.yaml; library — none (the tag is the version).
func codeVersion(root, kind string) string {
	switch kind {
	case "kotlin":
		data, err := os.ReadFile(filepath.Join(root, "app", "build.gradle.kts"))
		if err != nil {
			return ""
		}
		if m := regexp.MustCompile(`versionName\s*=\s*"([^"]+)"`).FindSubmatch(data); m != nil {
			return string(m[1])
		}
		return ""
	case "flutter":
		ver, _, ok := parsePubspecVersion(root)
		if !ok {
			return ""
		}
		return ver
	default:
		return ""
	}
}

// parsePubspecVersion reads the `version: X.Y.Z+N` line of a Flutter
// project's pubspec.yaml and returns the version and build number.
func parsePubspecVersion(root string) (string, int, bool) {
	data, err := os.ReadFile(filepath.Join(root, "pubspec.yaml"))
	if err != nil {
		return "", 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "version:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "version:"))
		v = strings.Trim(v, `"'`)
		parts := strings.SplitN(v, "+", 2)
		if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(parts[0]) {
			return "", 0, false
		}
		build := 0
		if len(parts) == 2 {
			build, _ = strconv.Atoi(parts[1])
		}
		return parts[0], build, true
	}
	return "", 0, false
}

// bumpPubspecVersion sets the pubspec version to the tag version and
// raises the build number by one (1.0.0+1 -> 1.0.1+2).
func bumpPubspecVersion(root, tag string) {
	tagVer := strings.TrimPrefix(tag, "v")
	ver, build, ok := parsePubspecVersion(root)
	if !ok {
		die("no version: line in pubspec.yaml — add one like `version: 1.0.0+1`.")
	}
	newVer := tagVer + "+" + strconv.Itoa(build+1)
	path := filepath.Join(root, "pubspec.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		die("pubspec.yaml not found in " + root)
	}
	re := regexp.MustCompile(`(?m)^version:\s*.*$`)
	out := re.ReplaceAllString(string(data), "version: "+newVer)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		die("could not update pubspec.yaml: " + err.Error())
	}
	say("pubspec version " + ver + "+" + strconv.Itoa(build) + " -> " + newVer)
}

// bumpKotlinVersion sets versionName in app/build.gradle.kts to the
// tag version and raises versionCode by one.
func bumpKotlinVersion(root, tag string) {
	tagVer := strings.TrimPrefix(tag, "v")
	path := filepath.Join(root, "app", "build.gradle.kts")
	data, err := os.ReadFile(path)
	if err != nil {
		die("app/build.gradle.kts not found in " + root)
	}
	verRe := regexp.MustCompile(`versionName\s*=\s*"([^"]+)"`)
	m := verRe.FindSubmatch(data)
	if m == nil {
		die("no versionName in app/build.gradle.kts — set one like versionName = \"1.0\", then retry.")
	}
	out := verRe.ReplaceAllString(string(data), `versionName = "`+tagVer+`"`)
	msg := "versionName " + string(m[1]) + " -> " + tagVer
	codeRe := regexp.MustCompile(`versionCode\s*=\s*([0-9]+)`)
	if cm := codeRe.FindSubmatch(data); cm != nil {
		n, _ := strconv.Atoi(string(cm[1]))
		out = codeRe.ReplaceAllString(out, "versionCode = "+strconv.Itoa(n+1))
		msg += ", versionCode " + string(cm[1]) + " -> " + strconv.Itoa(n+1)
	} else {
		fmt.Println("Note: no versionCode in app/build.gradle.kts — versionName updated only.")
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		die("could not update app/build.gradle.kts: " + err.Error())
	}
	say(msg)
}

// findReleaseApk finds the release APK Gradle just built
// (app/build/outputs/apk/release/app-release.apk, else any release
// APK there). It returns "" when there is none.
func findReleaseApk(root string) string {
	apk := filepath.Join(root, "app", "build", "outputs", "apk", "release", "app-release.apk")
	if _, err := os.Stat(apk); err == nil {
		return apk
	}
	var found string
	_ = filepath.Walk(filepath.Join(root, "app", "build", "outputs", "apk"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".apk") && strings.Contains(p, "release") && found == "" {
			found = p
		}
		return nil
	})
	return found
}

// findReleaseAar finds the release .aar Gradle just built
// (library/build/outputs/aar/release/library-release.aar, else any
// .aar there). It returns "" when there is none.
func findReleaseAar(root string) string {
	aarDir := filepath.Join(root, "library", "build", "outputs", "aar", "release")
	std := filepath.Join(aarDir, "library-release.aar")
	if _, err := os.Stat(std); err == nil {
		return std
	}
	entries, _ := os.ReadDir(aarDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".aar") {
			return filepath.Join(aarDir, e.Name())
		}
	}
	return ""
}

// removeLocalTag deletes a local git tag (best effort — used when a
// release fails after the tag was created, so the same version can
// be retried).
func removeLocalTag(tag string) {
	_ = exec.Command("git", "tag", "-d", tag).Run()
}

// checkApkStamp reads the APK's versionName with aapt and returns an
// error when it does not match the tag — uploading a wrong-version
// APK is exactly the wrong-version update loop. When aapt is missing
// it warns and skips, like before.
func checkApkStamp(apk, tag string) error {
	if aapt := findAapt(); aapt != "" {
		out, err := runCapture(aapt, "dump", "badging", apk)
		if err == nil {
			if m := regexp.MustCompile(`versionName='([^']+)'`).FindStringSubmatch(out); m != nil {
				stamp := m[1]
				fmt.Printf("APK stamp: versionName=%s  (tag wants %s)\n", stamp, strings.TrimPrefix(tag, "v"))
				if stamp != strings.TrimPrefix(tag, "v") {
					return fmt.Errorf("APK stamp mismatch — NOT uploading. This is exactly the wrong-version update loop; do not override it.")
				}
			}
		}
	} else {
		fmt.Println("WARNING: aapt not found, stamp check skipped. Verify once yourself:")
		fmt.Println("  adb install -r " + apk)
	}
	return nil
}

func isNewer(a, b string) bool {
	parse := func(s string) []int {
		s = strings.TrimPrefix(s, "v")
		parts := strings.Split(s, ".")
		out := []int{0, 0, 0}
		for i := 0; i < len(parts) && i < 3; i++ {
			n, _ := strconv.Atoi(parts[i])
			out[i] = n
		}
		return out
	}
	av, bv := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		if av[i] != bv[i] {
			return av[i] > bv[i]
		}
	}
	return false
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
