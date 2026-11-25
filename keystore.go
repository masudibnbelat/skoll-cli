package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// Release signing.
//
// A release APK that is not signed cannot be installed on a phone, so
// `angrboda build --release` and `angrboda release` make sure the
// project can sign before they build:
//
//   - One release keystore is created once, shared by all the user's
//     apps, at ~/.angrboda/keystore/release.keystore (alias "release").
//     Its passwords live next to it in release.properties (0600).
//   - The project is pointed at it: local.properties gets
//     releaseStoreFile / releaseStorePassword / releaseKeyAlias /
//     releaseKeyPassword (Kotlin), or android/key.properties gets
//     storeFile / storePassword / keyAlias / keyPassword (Flutter).
//     Both files are gitignored by the templates.
//   - A keystore the user already configured is never overwritten:
//     if the project already has signing keys, they are used as-is,
//     and if the keystore file they point at is gone, the command
//     stops and says so instead of silently making a new key.
//
// Losing the keystore means the app can never be updated on Google
// Play again, so every creation prints a BACK UP warning, and
// `angrboda keystore` shows the path + fingerprints + the same warning.

var keystoreCmd = &cobra.Command{
	Use:   "keystore",
	Short: "Show the release keystore (creates it if missing)",
	Long: `Shows the release keystore angrboda signs release builds with:
its path, alias and SHA-1 / SHA-256 fingerprints (the ones Google
Play and Firebase ask for). If no keystore exists yet, one is
created first (keytool, RSA 2048, valid 10000 days).

BACK UP the keystore file and its passwords. If it is lost, the app
can never be updated on Google Play again — a new keystore signs a
different app as far as the Play Store is concerned.

Move the keystore to another machine with:
  angrboda keystore export <folder>   (copies keystore + passwords)
  angrboda keystore import <folder>   (copies them back in)`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdKeystore()
	},
}

var keystoreExportCmd = &cobra.Command{
	Use:   "export <folder>",
	Short: "Copy the keystore + passwords to a folder (for backup or a new machine)",
	Long: `Copies release.keystore and release.properties into <folder>
(the folder is created if needed). Keep that folder safe — it
holds the passwords. On the other machine, run
` + "`angrboda keystore import <folder>`" + ` to put them back.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cmdKeystoreExport(args[0])
	},
}

var keystoreImportCmd = &cobra.Command{
	Use:   "import <folder>",
	Short: "Copy a keystore + passwords back in (from `angrboda keystore export`)",
	Long: `Copies release.keystore and release.properties from <folder>
into ~/.angrboda/keystore/, so this machine signs with the same key
as the machine the keystore came from. It refuses when a keystore
already exists here — delete it first if you really want to replace
it (and be sure: the old key can never be recovered).`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cmdKeystoreImport(args[0])
	},
}

func init() {
	keystoreCmd.AddCommand(keystoreExportCmd, keystoreImportCmd)
}

func keystoreDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".angrboda", "keystore")
}

func releaseKeystorePath() string {
	return filepath.Join(keystoreDir(), "release.keystore")
}

func releaseKeystorePropsPath() string {
	return filepath.Join(keystoreDir(), "release.properties")
}

// genPassword makes a 16-character random password (letters + digits)
// for a new keystore. It is stored, never shown.
func genPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		die("could not make a random password: " + err.Error())
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// readPropsFile reads a simple key=value properties file into a map.
// A missing file is not an error — it reads as empty.
func readPropsFile(path string) map[string]string {
	props := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return props
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if i := strings.IndexAny(line, "=:"); i > 0 {
			key := strings.TrimSpace(line[:i])
			val := strings.TrimSpace(line[i+1:])
			props[key] = val
		}
	}
	return props
}

// writePropsFile writes props as key=value lines (sorted, 0600 —
// these files hold keystore passwords).
func writePropsFile(path string, props map[string]string) {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k + "=" + props[k] + "\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		die("could not create " + filepath.Dir(path) + ": " + err.Error())
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		die("could not write " + path + ": " + err.Error())
	}
}

// updatePropsFile sets the given keys in a properties file, keeping
// every other line byte-for-byte (sdk.dir, comments, ...). Keys that
// are not there yet are appended. A missing file is created (0600).
func updatePropsFile(path string, updates map[string]string) {
	remaining := map[string]string{}
	for k, v := range updates {
		remaining[k] = v
	}
	var out []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if i := strings.IndexAny(trimmed, "=:"); i > 0 {
				if v, ok := remaining[strings.TrimSpace(trimmed[:i])]; ok {
					out = append(out, strings.TrimSpace(trimmed[:i])+"="+v)
					delete(remaining, strings.TrimSpace(trimmed[:i]))
					continue
				}
			}
			out = append(out, line)
		}
		// Drop the trailing empty line the split adds, so the appended
		// keys do not start after a blank line.
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
	}
	keys := make([]string, 0, len(remaining))
	for k := range remaining {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+remaining[k])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		die("could not create " + filepath.Dir(path) + ": " + err.Error())
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0o600); err != nil {
		die("could not write " + path + ": " + err.Error())
	}
}

// findKeytool finds keytool (ships with every JDK): JAVA_HOME first,
// then the angrboda Temurin install, then PATH.
func findKeytool() string {
	if jh := os.Getenv("JAVA_HOME"); jh != "" {
		p := filepath.Join(jh, "bin", "keytool")
		if runtimeIsWindows() {
			p += ".exe"
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	home, _ := os.UserHomeDir()
	matches, _ := filepath.Glob(filepath.Join(home, ".angrboda", "jdk", "*", "bin", "keytool"))
	if len(matches) > 0 {
		sort.Strings(matches)
		return matches[len(matches)-1]
	}
	if p, err := lookPath("keytool"); err == nil {
		return p
	}
	return ""
}

// ensureSharedKeystore creates the shared release keystore if it
// does not exist yet and returns its path. The passwords are stored
// in release.properties next to it — the only place they live.
func ensureSharedKeystore() string {
	ksPath := releaseKeystorePath()
	if _, err := os.Stat(ksPath); err == nil {
		return ksPath
	}
	keytool := findKeytool()
	if keytool == "" {
		ensureJava()
		keytool = findKeytool()
		if keytool == "" {
			die("keytool not found — Java 17+ is needed to create a release keystore.")
		}
	}
	pw := genPassword()
	if err := os.MkdirAll(keystoreDir(), 0o700); err != nil {
		die("could not create " + keystoreDir() + ": " + err.Error())
	}
	args := []string{
		"-genkeypair", "-keystore", ksPath, "-alias", "release",
		"-keyalg", "RSA", "-keysize", "2048", "-validity", "10000",
		"-storepass", pw, "-keypass", pw,
		"-dname", "CN=Android Developer, OU=Development, O=Developer, C=US",
		"-noprompt",
	}
	if _, err := runCapture(keytool, args...); err != nil {
		die("keytool failed to create the release keystore: " + err.Error())
	}
	// The keystore holds a private key — only its owner may read it.
	_ = os.Chmod(ksPath, 0o600)
	writePropsFile(releaseKeystorePropsPath(), map[string]string{
		"storeFile":     ksPath,
		"storePassword": pw,
		"keyAlias":      "release",
		"keyPassword":   pw,
	})
	okLine("Release keystore created: " + ksPath)
	warnLine("BACK UP this file + its passwords (in " + releaseKeystorePropsPath() + "). Losing it means you can never update this app on Google Play again.")
	return ksPath
}

// ensureReleaseKeystore makes sure root can sign a release build:
// an already-configured project is left alone; otherwise the shared
// keystore is created (if needed) and the project is pointed at it.
// It never overwrites keys the user already set.
func ensureReleaseKeystore(root string) {
	flutter := isFlutterProject(root)
	var propsPath string
	if flutter {
		propsPath = filepath.Join(root, "android", "key.properties")
	} else {
		propsPath = filepath.Join(root, "local.properties")
	}
	existing := readPropsFile(propsPath)
	if flutter {
		if existing["storeFile"] != "" && existing["storePassword"] != "" && existing["keyAlias"] != "" && existing["keyPassword"] != "" {
			if _, err := os.Stat(existing["storeFile"]); err == nil {
				say("Release signing: " + existing["storeFile"])
				return
			}
			die("release keystore file is missing: " + existing["storeFile"] + " — restore it, or delete android/key.properties and re-run so a new one is created.")
		}
	} else {
		if existing["releaseStoreFile"] != "" && existing["releaseStorePassword"] != "" && existing["releaseKeyAlias"] != "" && existing["releaseKeyPassword"] != "" {
			if _, err := os.Stat(existing["releaseStoreFile"]); err == nil {
				say("Release signing: " + existing["releaseStoreFile"])
				return
			}
			die("release keystore file is missing: " + existing["releaseStoreFile"] + " — restore it, or delete the release* lines from local.properties and re-run so a new one is created.")
		}
	}
	ksPath := ensureSharedKeystore()
	shared := readPropsFile(releaseKeystorePropsPath())
	if shared["storePassword"] == "" || shared["keyPassword"] == "" {
		die("release keystore exists at " + ksPath + " but its password is unknown (no " + releaseKeystorePropsPath() + ") — delete the keystore file and re-run to create a fresh one.")
	}
	if flutter {
		updatePropsFile(propsPath, map[string]string{
			"storeFile":     ksPath,
			"storePassword": shared["storePassword"],
			"keyAlias":      shared["keyAlias"],
			"keyPassword":   shared["keyPassword"],
		})
	} else {
		updatePropsFile(propsPath, map[string]string{
			"releaseStoreFile":     ksPath,
			"releaseStorePassword": shared["storePassword"],
			"releaseKeyAlias":      shared["keyAlias"],
			"releaseKeyPassword":   shared["keyPassword"],
		})
	}
	say("Release signing: " + ksPath + " (written to " + filepath.Base(propsPath) + ")")
}

func cmdKeystore() {
	ksPath := ensureSharedKeystore()
	shared := readPropsFile(releaseKeystorePropsPath())
	fmt.Println("Release keystore: " + ksPath)
	if shared["keyAlias"] != "" {
		fmt.Println("Alias: " + shared["keyAlias"])
	}
	keytool := findKeytool()
	if keytool == "" {
		ensureJava()
		keytool = findKeytool()
	}
	if keytool != "" && shared["storePassword"] != "" {
		out, err := runCapture(keytool, "-list", "-v", "-keystore", ksPath, "-storepass", shared["storePassword"])
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "SHA1:") || strings.HasPrefix(line, "SHA256:") {
					fmt.Println(line)
				}
			}
		} else {
			fmt.Println("WARNING: keytool could not read the keystore (" + err.Error() + ")")
		}
	}
	fmt.Println("Passwords: " + releaseKeystorePropsPath())
	warnLine("BACK UP the keystore file + its passwords. Losing it means you can never update this app on Google Play again.")
}

// cmdKeystoreExport copies the keystore + its passwords into dir
// (created if needed), so the user can back them up or carry them
// to another machine. Both files keep 0600.
func cmdKeystoreExport(dir string) {
	ksPath := ensureSharedKeystore()
	propsPath := releaseKeystorePropsPath()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		die("could not create " + dir + ": " + err.Error())
	}
	for _, src := range []string{ksPath, propsPath} {
		dst := filepath.Join(dir, filepath.Base(src))
		data, err := os.ReadFile(src)
		if err != nil {
			die("could not read " + src + ": " + err.Error())
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			die("could not write " + dst + ": " + err.Error())
		}
	}
	okLine("Keystore exported to " + dir + " (release.keystore + release.properties)")
	warnLine("Keep this folder safe — it holds the keystore passwords. On the other machine: angrboda keystore import " + dir)
}

// cmdKeystoreImport copies the keystore + its passwords from dir
// back into ~/.angrboda/keystore/. It refuses when a keystore
// already exists here: replacing the signing key silently would
// break Play Store updates for every app signed with the old one.
func cmdKeystoreImport(dir string) {
	if _, err := os.Stat(releaseKeystorePath()); err == nil {
		die("a release keystore already exists here (" + releaseKeystorePath() + ") — delete it first if you really want to replace it, then re-run the import.")
	}
	for _, name := range []string{"release.keystore", "release.properties"} {
		src := filepath.Join(dir, name)
		if _, err := os.Stat(src); err != nil {
			die("missing " + src + " — that folder is not a keystore export (run `angrboda keystore export` on the other machine first).")
		}
	}
	if err := os.MkdirAll(keystoreDir(), 0o700); err != nil {
		die("could not create " + keystoreDir() + ": " + err.Error())
	}
	for _, name := range []string{"release.keystore", "release.properties"} {
		src := filepath.Join(dir, name)
		dst := filepath.Join(keystoreDir(), name)
		data, err := os.ReadFile(src)
		if err != nil {
			die("could not read " + src + ": " + err.Error())
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			die("could not write " + dst + ": " + err.Error())
		}
	}
	okLine("Keystore imported — this machine now signs with " + releaseKeystorePath())
}

// findApksigner finds apksigner in the newest build-tools, like
// findAapt does for aapt. It returns "" when there is no SDK.
func findApksigner() string {
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		return ""
	}
	pattern := filepath.Join(sdk, "build-tools", "*", "apksigner")
	if runtimeIsWindows() {
		pattern += ".bat"
	}
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}

// certIsDebug reports whether a signer certificate DN is the
// Android debug key ("CN=Android Debug, O=Android, C=US") — the key
// every debug build uses. A release APK signed with it is not a real
// release signature.
func certIsDebug(dn string) bool {
	return strings.Contains(dn, "CN=Android Debug")
}

// verifyApkSigned checks the APK is really signed (apksigner verify)
// and prints the signer certificate, returning an error when it is
// not signed, or signed with the debug key. An unsigned release APK
// cannot be installed, so a failed check stops the release (and
// `angrboda build --release`). When apksigner is missing it warns
// and skips, like the aapt stamp check does.
func verifyApkSigned(apk string) error {
	apksigner := findApksigner()
	if apksigner == "" {
		fmt.Println("WARNING: apksigner not found, signature check skipped. Verify once yourself:")
		fmt.Println("  apksigner verify " + apk)
		return nil
	}
	out, err := runCapture(apksigner, "verify", "--print-certs", apk)
	if err != nil {
		return fmt.Errorf("APK is NOT signed: %s — a release APK must be signed to install. Check the signing config, then retry.", apk)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Signer #1 certificate DN:") {
			dn := strings.TrimSpace(strings.TrimPrefix(line, "Signer #1 certificate DN:"))
			if certIsDebug(dn) {
				return fmt.Errorf("APK is signed with the DEBUG key (%s), not a release key — check the signing config (local.properties / android/key.properties), then retry.", dn)
			}
			okLine("Signature OK: " + dn)
			return nil
		}
	}
	okLine("Signature OK: " + apk + " is signed")
	return nil
}
