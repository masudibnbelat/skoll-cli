package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Latest STABLE versions for `angrboda new`: looked up live, so a
// project created today always uses today's latest stable stack.
// Maven's <release> field cannot be trusted here — it currently
// points at an AGP alpha (9.6.0-alpha06) and a Kotlin RC
// (2.5.0-RC2) — so the full <versions> list is read and the highest
// stable version is picked. When there is no internet (or a lookup
// fails), the pinned versions below are used — the set this template
// was verified with.
type appVersions struct {
	gradle string
	agp    string
	kotlin string
	bom    string
	live   int // how many of the 4 were fetched live
}

var pinnedVersions = appVersions{
	gradle: "9.3.1",
	agp:    "9.1.1",
	kotlin: "2.2.10",
	bom:    "2025.09.00",
}

// Flutter's own pinned template versions (flutter_tools stable):
// Flutter only supports these AGP/Gradle/Kotlin combos, so the
// latest Gradle 9.x must NOT be used for a Flutter project.
// (Flutter 3.50.1 stable, Dart 3.14.0.)
var flutterPinnedVersions = appVersions{
	gradle: "9.5.0",
	agp:    "9.3.1",
	kotlin: "2.4.20",
}

var versionRe = regexp.MustCompile(`^[0-9][0-9A-Za-z.\-+]*$`)

func resolveLatestVersions() appVersions {
	v := pinnedVersions
	client := &http.Client{Timeout: 5 * time.Second}
	if s := fetchGradleVersion(client); s != "" {
		v.gradle = s
		v.live++
	}
	if s := fetchMavenLatest(client, "https://dl.google.com/dl/android/maven2/com/android/application/com.android.application.gradle.plugin/maven-metadata.xml"); s != "" {
		v.agp = s
		v.live++
	}
	if s := fetchMavenLatest(client, "https://repo.maven.apache.org/maven2/org/jetbrains/kotlin/kotlin-gradle-plugin/maven-metadata.xml"); s != "" {
		v.kotlin = s
		v.live++
	}
	if s := fetchMavenLatest(client, "https://dl.google.com/dl/android/maven2/androidx/compose/compose-bom/maven-metadata.xml"); s != "" {
		v.bom = s
		v.live++
	}
	return v
}

// describe says which versions will be used, and whether they are
// live or the offline pinned set.
func (v appVersions) describe() string {
	base := fmt.Sprintf("Gradle %s, AGP %s, Kotlin %s, Compose BOM %s", v.gradle, v.agp, v.kotlin, v.bom)
	switch {
	case v.live == 4:
		return "Latest versions: " + base
	case v.live == 0:
		return "Offline — pinned versions: " + base
	default:
		return "Versions: " + base + " (some pinned — no internet for those)"
	}
}

// describeWithoutBom is describe() without the Compose BOM part,
// for the empty and library templates, which do not use Compose.
func (v appVersions) describeWithoutBom() string {
	base := fmt.Sprintf("Gradle %s, AGP %s, Kotlin %s", v.gradle, v.agp, v.kotlin)
	switch {
	case v.live == 4:
		return "Latest versions: " + base
	case v.live == 0:
		return "Offline — pinned versions: " + base
	default:
		return "Versions: " + base + " (some pinned — no internet for those)"
	}
}

// applyVersionPins overrides the looked-up versions with the
// --gradle/--agp/--kotlin/--bom flags, when given. A bad shape dies
// here instead of failing halfway through a Gradle build.
func applyVersionPins(v appVersions) appVersions {
	if newGradlePin != "" {
		if !versionRe.MatchString(newGradlePin) {
			die("bad --gradle '" + newGradlePin + "' — use a version like 9.3.1")
		}
		v.gradle = newGradlePin
	}
	if newAgpPin != "" {
		if !versionRe.MatchString(newAgpPin) {
			die("bad --agp '" + newAgpPin + "' — use a version like 9.1.1")
		}
		v.agp = newAgpPin
	}
	if newKotlinPin != "" {
		if !versionRe.MatchString(newKotlinPin) {
			die("bad --kotlin '" + newKotlinPin + "' — use a version like 2.2.10")
		}
		v.kotlin = newKotlinPin
	}
	if newBomPin != "" {
		if !regexp.MustCompile(`^[0-9]{4}\.[0-9]{2}\.[0-9]{2}$`).MatchString(newBomPin) {
			die("bad --bom '" + newBomPin + "' — use a date like 2025.09.00")
		}
		v.bom = newBomPin
	}
	return v
}

// anyPinSet reports whether any --gradle/--agp/--kotlin/--bom flag
// was given.
func anyPinSet() bool {
	return newGradlePin != "" || newAgpPin != "" || newKotlinPin != "" || newBomPin != ""
}

// fetchGradleVersion reads https://services.gradle.org/versions/current
// and returns the current Gradle version, e.g. "9.3.1".
func fetchGradleVersion(client *http.Client) string {
	resp, err := client.Get("https://services.gradle.org/versions/current")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var data struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}
	if !versionRe.MatchString(data.Version) {
		return ""
	}
	return data.Version
}

// fetchMavenLatest reads a maven-metadata.xml and returns the
// highest STABLE version in its <versions> list, e.g. "9.5.2".
// Pre-release versions (alpha, beta, RC, ...) are skipped, because
// a starter template must build on day one.
func fetchMavenLatest(client *http.Client, url string) string {
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var data struct {
		Versioning struct {
			Latest   string   `xml:"latest"`
			Release  string   `xml:"release"`
			Versions []string `xml:"versions>version"`
		} `xml:"versioning"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}
	best := ""
	for _, v := range data.Versioning.Versions {
		if isStableVersion(v) && compareVersions(v, best) > 0 {
			best = v
		}
	}
	if best != "" {
		return best
	}
	// No <versions> list: fall back to <release>/<latest> if stable.
	for _, s := range []string{data.Versioning.Release, data.Versioning.Latest} {
		if isStableVersion(s) {
			return s
		}
	}
	return ""
}

// isStableVersion reports whether v is a stable release: no
// alpha/beta/RC/preview/snapshot/dev/EAP/milestone markers.
func isStableVersion(v string) bool {
	if !versionRe.MatchString(v) {
		return false
	}
	lower := strings.ToLower(v)
	for _, marker := range []string{"alpha", "beta", "rc", "cr", "preview", "snapshot", "dev", "eap", "milestone"} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	// "-M1", "-B1" style milestone/beta suffixes.
	if preSuffixRe.MatchString(lower) {
		return false
	}
	return true
}

var preSuffixRe = regexp.MustCompile(`-[mb][0-9]`)

// compareVersions compares two dot-separated numeric versions
// ("9.5.2" > "9.1.1", "2026.11.01" > "2025.09.00"). It returns -1,
// 0 or 1. An empty string loses to any real version.
func compareVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		an, bn := 0, 0
		if i < len(as) {
			fmt.Sscanf(as[i], "%d", &an)
		}
		if i < len(bs) {
			fmt.Sscanf(bs[i], "%d", &bn)
		}
		if an != bn {
			if an < bn {
				return -1
			}
			return 1
		}
	}
	return 0
}
