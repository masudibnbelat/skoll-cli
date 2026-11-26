// Location: ui.go
package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Terminal UI: colors, step/done lines, and the download progress bar.
//
// Everything degrades gracefully when stdout is piped (CI, logs):
// no colors, no \r lines, plain == / STOP: lines instead. Set
// NO_COLOR=1 to force plain output even on a terminal.

// colorOn reports whether ANSI colors should be used for f.
func colorOn(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(f)
}

// colorEnabled reports whether stdout gets colors.
func colorEnabled() bool {
	return colorOn(os.Stdout)
}

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiGreen   = "\x1b[32m"
	ansiCyan    = "\x1b[36m"
	ansiYellow  = "\x1b[33m"
	ansiRed     = "\x1b[31m"
	ansiMagenta = "\x1b[35m"
	ansiDim     = "\x1b[2m"
)

func paint(code, s string) string {
	if !colorEnabled() {
		return s
	}
	return code + s + ansiReset
}

func green(s string) string   { return paint(ansiGreen, s) }
func cyan(s string) string    { return paint(ansiCyan, s) }
func yellow(s string) string  { return paint(ansiYellow, s) }
func red(s string) string     { return paint(ansiRed, s) }
func magenta(s string) string { return paint(ansiMagenta, s) }
func bold(s string) string    { return paint(ansiBold, s) }
func dim(s string) string     { return paint(ansiDim, s) }

// visibleLen counts the characters a line takes on screen, ignoring
// ANSI escape sequences, so padding works on colored lines too.
func visibleLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

// printBox prints lines inside a green box:
//
//	┌──────────────────────────────────┐
//	│ ✓ Created MyApp in MyApp (42 files) │
//	│ Package: com.example.myapp       │
//	│ Next: cd MyApp && skoll build │
//	└──────────────────────────────────┘
//
// Plain (no colors) when piped; the box is still drawn.
func printBox(lines []string) {
	width := 0
	for _, l := range lines {
		if v := visibleLen(l); v > width {
			width = v
		}
	}
	fmt.Println(green("┌" + strings.Repeat("─", width+2) + "┐"))
	for _, l := range lines {
		fmt.Println(green("│") + " " + l + strings.Repeat(" ", width-visibleLen(l)) + " " + green("│"))
	}
	fmt.Println(green("└" + strings.Repeat("─", width+2) + "┘"))
}

// stepLine prints a "now doing" line: → msg
func stepLine(msg string) {
	fmt.Println(cyan("→") + " " + msg)
}

// okLine prints a finished line: ✓ msg
func okLine(msg string) {
	fmt.Println(green("✓") + " " + msg)
}

// warnLine prints a warning line: ⚠ msg
func warnLine(msg string) {
	fmt.Println(yellow("⚠") + " " + msg)
}

// doctorLine prints one environment-check line: ✓ label: value (green)
// or ✗ label: MISSING (...) (red) on a terminal; plain "label: value"
// when piped.
func doctorLine(label, value string, present bool) {
	if !colorEnabled() {
		fmt.Println(label + ": " + value)
		return
	}
	if present {
		fmt.Println(green("✓") + " " + label + ": " + value)
	} else {
		fmt.Println(red("✗") + " " + label + ": " + value)
	}
}

// formatBytes renders a byte count the way a person reads it:
// "512 B", "9.4 MB", "1.5 GB". Whole numbers drop the ".0".
func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n)
	i := -1
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	// One decimal, dropping a ".0" (112.0 KB reads worse than 112 KB).
	num := fmt.Sprintf("%.1f", v)
	num = strings.TrimSuffix(num, ".0")
	return num + " " + units[i]
}

// formatDuration renders a duration the way a person reads it:
// "45s", "1m 05s", "1h 02m".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
}

const progressBarWidth = 30

// progressLine renders one download line (no trailing newline):
//
//	██████████████░░░░░░░░░░░░░░░░  45%  690 MB / 1.5 GB  12.4 MB/s  ETA 1m 05s
//
// speed <= 0 means "not measured yet" — speed and ETA show as ….
func progressLine(written, total int64, speed float64, eta time.Duration) string {
	pct := 0
	if total > 0 {
		pct = int(written * 100 / total)
		if pct > 100 {
			pct = 100
		}
	}
	filled := pct * progressBarWidth / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", progressBarWidth-filled)
	if colorEnabled() {
		bar = cyan(strings.Repeat("█", filled)) + dim(strings.Repeat("░", progressBarWidth-filled))
	}
	speedStr := "…"
	etaStr := "…"
	if speed > 0 {
		speedStr = formatBytes(int64(speed)) + "/s"
		etaStr = "ETA " + formatDuration(eta)
	}
	return fmt.Sprintf("%s  %3d%%  %s / %s  %s  %s", bar, pct, formatBytes(written), formatBytes(total), speedStr, etaStr)
}

// progressLineUnknown renders the live line when the total size is
// unknown: downloaded size + speed, no percent, no ETA.
func progressLineUnknown(written int64, speed float64) string {
	speedStr := "…"
	if speed > 0 {
		speedStr = formatBytes(int64(speed)) + "/s"
	}
	return fmt.Sprintf("%s  %s", formatBytes(written), speedStr)
}

// downloadBar draws a live progress line for one download and the
// finished line when it is done. Created by downloadFileProgress.
type downloadBar struct {
	label string
	total int64
	start time.Time
	shown bool
}

func newDownloadBar(label string, total int64) *downloadBar {
	return &downloadBar{label: label, total: total, start: time.Now()}
}

// render draws (or redraws) the line for the current byte count: the
// full bar when the total size is known, size + speed when it is not.
func (b *downloadBar) render(written int64) {
	elapsed := time.Since(b.start)
	var speed float64
	if elapsed >= 500*time.Millisecond && written > 0 {
		speed = float64(written) / elapsed.Seconds()
	}
	var line string
	if b.total > 0 {
		var eta time.Duration
		if speed > 0 && b.total > written {
			eta = time.Duration(float64(b.total-written)/speed) * time.Second
		}
		line = progressLine(written, b.total, speed, eta)
	} else {
		line = progressLineUnknown(written, speed)
	}
	fmt.Fprintf(os.Stdout, "\r\x1b[K%s", line)
	b.shown = true
}

// finish clears the bar line and prints the done line:
// ✓ <label> downloaded — 1.5 GB in 2m 03s (12.4 MB/s)
func (b *downloadBar) finish(written int64) {
	elapsed := time.Since(b.start)
	if b.shown {
		fmt.Fprint(os.Stdout, "\r\x1b[K")
	}
	avg := ""
	if elapsed >= time.Second && written > 0 {
		avg = " (" + formatBytes(int64(float64(written)/elapsed.Seconds())) + "/s)"
	}
	fmt.Fprintf(os.Stdout, "%s %s downloaded — %s in %s%s\n", green("✓"), b.label, formatBytes(written), formatDuration(elapsed), avg)
}
