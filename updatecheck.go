// Location: updatecheck.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Automatic update notice: every skoll command (except update,
// version, help and completion) shows a small box when a newer
// release exists — so the user never has to remember to check.
// The latest release is re-checked at most every 6 hours, in the
// background, so the check never slows a command down and never
// fails one: the notice comes from the cached result, and an
// offline machine just shows nothing. Set
// SKOLL_NO_UPDATE_CHECK=1 to turn it off entirely.

// updateCheckInterval: how often the latest release is re-checked.
const updateCheckInterval = 6 * time.Hour

type updateCache struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
}

func updateCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".skoll", "update-check.json")
}

func readUpdateCache(path string) updateCache {
	var c updateCache
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c)
	return c
}

func writeUpdateCache(path string, c updateCache) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// updateAvailable reports whether latest is newer than current.
func updateAvailable(latest, current string) bool {
	if latest == "" {
		return false
	}
	return compareVersions(strings.TrimPrefix(latest, "v"), current) > 0
}

// needsUpdateCheck reports whether the cached check is stale
// (missing or older than a day).
func needsUpdateCheck(c updateCache, now time.Time) bool {
	if c.Latest == "" {
		return true
	}
	return now.Sub(c.CheckedAt) >= updateCheckInterval
}

// skipUpdateNotice lists the commands that never show the box:
// update already handles its own messaging, version is often used
// in scripts, and help / completion output must stay clean.
var skipUpdateNotice = map[string]bool{
	"update":           true,
	"version":          true,
	"help":             true,
	"completion":       true,
	"__complete":       true,
	"__completeNoDesc": true,
}

// maybeNotifyUpdate shows the update box when the cached latest
// release is newer than this binary, and refreshes the cache in
// the background when it is stale. It runs from the root command's
// PersistentPreRun, so before every command except the skipped
// ones — and never on a piped run, so scripts stay clean.
func maybeNotifyUpdate(cmd *cobra.Command) {
	if os.Getenv("SKOLL_NO_UPDATE_CHECK") != "" {
		return
	}
	if skipUpdateNotice[cmd.Name()] {
		return
	}
	if !colorEnabled() {
		return
	}
	path := updateCachePath()
	cached := readUpdateCache(path)
	if updateAvailable(cached.Latest, version) {
		printUpdateNotice(cached.Latest)
	}
	if needsUpdateCheck(cached, time.Now()) {
		refreshUpdateCacheAsync(path)
	}
}

// refreshUpdateCacheAsync fetches the latest release tag in the
// background and stores it. It never blocks the command and never
// reports errors — a failed check just leaves the old cache.
func refreshUpdateCacheAsync(path string) {
	go func() {
		latest, err := latestReleaseTag()
		if err != nil {
			return
		}
		writeUpdateCache(path, updateCache{Latest: latest, CheckedAt: time.Now()})
	}()
}

// printUpdateNotice prints the update box:
//
//	┌───────────────────────────────────────────────┐
//	│ ✨  Update available: v2.0.0  (you have v1.0.5) │
//	│     Run skoll update to install it.           │
//	└───────────────────────────────────────────────┘
func printUpdateNotice(latest string) {
	fmt.Println()
	printBox([]string{
		yellow("✨") + "  Update available: " + bold(green(latest)) + dim("  (you have v"+version+")"),
		"    Run " + cyan("skoll update") + dim(" to install it."),
	})
}
