// Location: update.go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var updateCheck bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update skoll itself to the latest release",
	Long: `Checks the latest skoll release on GitHub and, when a newer
version exists, downloads the matching binary, verifies its SHA256
against the release checksums, and replaces the current binary.
With --check it only reports whether an update exists.

The latest tag is read from the GitHub web redirect (no API call),
so the unauthenticated API rate limit never blocks it.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmdUpdate(updateCheck)
	},
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Only check for an update, do not install it")
}

func cmdUpdate(checkOnly bool) {
	latest, err := latestReleaseTag()
	if err != nil {
		die("could not find the latest release (" + err.Error() + "). Check your internet, or wait out GitHub's rate limit.")
	}
	if cmp := compareVersions(strings.TrimPrefix(latest, "v"), version); cmp < 0 {
		fmt.Println(green("✓") + " You have " + bold("v"+version) + ", newer than the latest release (" + latest + ") — nothing to update.")
		return
	} else if cmp == 0 {
		fmt.Println(green("✓") + " " + bold("skoll "+version) + " is already the latest (" + latest + ").")
		return
	}
	printBox([]string{
		cyan("↑") + "  Update available: " + bold(green(latest)),
		"  You have " + bold("v"+version) + dim("  →  "+latest),
	})
	if checkOnly {
		return
	}
	asset := updateAssetName()
	url := "https://github.com/masudibnbelat/skoll-cli/releases/download/" + latest + "/" + asset
	tmpDir, err := os.MkdirTemp("", "skoll-update-*")
	if err != nil {
		die(err.Error())
	}
	defer os.RemoveAll(tmpDir)
	tmpBin := filepath.Join(tmpDir, asset)
	if err := downloadFileProgress(url, tmpBin, "skoll "+latest); err != nil {
		die("could not download " + url + " (" + err.Error() + ")")
	}
	if err := verifyChecksum(tmpBin, latest, asset); err != nil {
		die("checksum check failed (" + err.Error() + ") — refusing to install. Delete the download and retry.")
	}
	okLine("Checksum OK (SHA256)")
	if err := replaceCurrentBinary(tmpBin); err != nil {
		die("could not replace the current binary (" + err.Error() + "). If it lives in a protected folder, reinstall with the installer instead.")
	}
	printBox([]string{
		green("✓") + "  Updated to " + bold(green(latest)),
		dim("  Run `skoll --version` to confirm."),
	})
}

func updateAssetName() string {
	name := "skoll-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// latestReleaseTag reads the latest tag from the GitHub web redirect
// (no API call, so no rate limit), falling back to the API.
func latestReleaseTag() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get("https://github.com/masudibnbelat/skoll-cli/releases/latest")
	if err == nil {
		defer resp.Body.Close()
		if loc := resp.Header.Get("Location"); loc != "" {
			if i := strings.LastIndex(loc, "/tag/"); i >= 0 {
				return loc[i+5:], nil
			}
		}
	}
	// Fallback: the API (GITHUB_TOKEN honoured when set).
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/masudibnbelat/skoll-cli/releases/latest", nil)
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp2, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		return "", fmt.Errorf("GitHub answered HTTP %s", resp2.Status)
	}
	var data struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.TagName == "" {
		return "", fmt.Errorf("no tag in API response")
	}
	return data.TagName, nil
}

// verifyChecksum downloads checksums.txt for the release and checks
// the binary's SHA256 against it.
func verifyChecksum(binPath, tag, asset string) error {
	url := "https://github.com/masudibnbelat/skoll-cli/releases/download/" + tag + "/checksums.txt"
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("checksums.txt: HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(body), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == asset {
			want = f[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum for %s in checksums.txt", asset)
	}
	f, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("SHA256 mismatch for %s", asset)
	}
	return nil
}

// moveFile moves src to dst. A plain rename is tried first; when it
// fails because src and dst are on different filesystems (e.g. the
// download in /tmp, which is often a tmpfs, and the install dir under
// $HOME — rename then dies with "invalid cross-device link"), it falls
// back to copying the bytes and removing the source.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Rename failed (usually EXDEV: src and dst are on different
	// filesystems) — copy the bytes instead.
	return copyExecutable(src, dst)
}

// copyExecutable moves src to dst, even across filesystems and even
// when dst is the running binary: it copies src to a temp file in
// dst's own directory (same filesystem) and renames that over dst.
// Writing to dst directly would fail with "text file busy" while
// the binary is running, and renaming src to dst across filesystems
// fails with "invalid cross-device link" — the temp file in dst's
// directory avoids both. (The copyFile in release.go is for build
// artifacts: it keeps the source and does not set the executable
// bit, so it cannot be reused here.)
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".skoll-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// replaceCurrentBinary swaps the running binary for the new one.
// Unix: rename over the running file works. Windows: the running
// .exe is locked, so the current file is first renamed aside
// (renaming a running exe is allowed), the new file is moved in,
// and the old one is removed on the next start (see main.go).
func replaceCurrentBinary(newBin string) error {
	current, err := os.Executable()
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		old := current + ".old"
		_ = os.Remove(old)
		if err := os.Rename(current, old); err != nil {
			return err
		}
		if err := moveFile(newBin, current); err != nil {
			// Roll back: put the old binary back.
			_ = os.Rename(old, current)
			return err
		}
		return nil
	}
	if err := os.Chmod(newBin, 0755); err != nil {
		return err
	}
	return moveFile(newBin, current)
}
