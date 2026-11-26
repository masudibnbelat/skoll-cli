// Location: prompt.go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

// Arrow-key menus for the interactive questions (project type in
// `new` / `init`). No TUI framework — one small menu on top of the
// ANSI colors ui.go already has, and golang.org/x/term for raw mode
// (Linux, macOS and Windows).
//
// Keys: ↑ / ↓ or j / k to move, 1-4 to jump, Enter to confirm,
// Esc or Ctrl+C to cancel. When stdin is piped, or the terminal
// cannot do raw mode, the caller falls back to the old typed
// question, so scripts keep working.
//
// Get the dependency once, then it builds offline:
//   go get golang.org/x/term
//   go mod tidy

var errNotTerminal = errors.New("not a terminal")
var errCancelled = errors.New("cancelled")

// selectChoice is one row of a menu: the label, a dim description,
// the value returned on Enter, and the accent color for the label.
type selectChoice struct {
	label  string
	desc   string
	value  string
	accent func(string) string
}

type keyKind int

const (
	keyUp keyKind = iota
	keyDown
	keyEnter
	keyCancel
	keyNumber
	keyOther
)

type keyPress struct {
	kind keyKind
	num  int
}

// readKey reads one key press from r: arrows, Enter, Esc, Ctrl+C,
// digits and j/k. Arrow keys arrive as a 3-byte escape sequence;
// a lone Esc (nothing follows within 60ms) means cancel.
func readKey(r *bufio.Reader) keyPress {
	b, err := r.ReadByte()
	if err != nil {
		return keyPress{kind: keyCancel}
	}
	switch b {
	case '\r', '\n':
		return keyPress{kind: keyEnter}
	case 0x03, 0x04: // Ctrl+C, Ctrl+D
		return keyPress{kind: keyCancel}
	case 0x1b:
		b2, ok := readByteTimeout(r, 60*time.Millisecond)
		if !ok {
			return keyPress{kind: keyCancel}
		}
		if b2 == '[' || b2 == 'O' {
			b3, ok := readByteTimeout(r, 60*time.Millisecond)
			if !ok {
				return keyPress{kind: keyOther}
			}
			switch b3 {
			case 'A':
				return keyPress{kind: keyUp}
			case 'B':
				return keyPress{kind: keyDown}
			}
		}
		return keyPress{kind: keyOther}
	case 'j':
		return keyPress{kind: keyDown}
	case 'k':
		return keyPress{kind: keyUp}
	default:
		if b >= '1' && b <= '9' {
			return keyPress{kind: keyNumber, num: int(b - '0')}
		}
		return keyPress{kind: keyOther}
	}
}

// readByteTimeout reads one byte, giving up after d. The timeout
// only fires for a lone Esc — arrow keys always arrive together,
// so a real arrow press never waits.
func readByteTimeout(r *bufio.Reader, d time.Duration) (byte, bool) {
	type result struct {
		b   byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		b, err := r.ReadByte()
		ch <- result{b, err}
	}()
	select {
	case v := <-ch:
		if v.err != nil {
			return 0, false
		}
		return v.b, true
	case <-time.After(d):
		return 0, false
	}
}

// selectOne shows an arrow-key menu and returns the chosen index.
// The menu is drawn in place: moving re-renders the same lines, so
// it animates instead of scrolling. On Enter it clears the menu and
// prints one confirmation line; on Esc/Ctrl+C it clears the menu
// and returns errCancelled. When the terminal cannot do this it
// returns errNotTerminal and the caller falls back to typing.
func selectOne(reader *bufio.Reader, title string, choices []selectChoice) (int, error) {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) || os.Getenv("TERM") == "dumb" {
		return -1, errNotTerminal
	}
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return -1, errNotTerminal
	}
	defer term.Restore(fd, oldState)
	fmt.Fprint(os.Stdout, "\x1b[?25l")       // hide cursor
	defer fmt.Fprint(os.Stdout, "\x1b[?25h") // show cursor

	total := len(choices) + 2 // title + choices + hint
	selected := 0
	renderMenu(title, choices, selected)
	for {
		k := readKey(reader)
		changed := false
		switch k.kind {
		case keyUp:
			selected = (selected - 1 + len(choices)) % len(choices)
			changed = true
		case keyDown:
			selected = (selected + 1) % len(choices)
			changed = true
		case keyNumber:
			if k.num >= 1 && k.num <= len(choices) {
				selected = k.num - 1
				changed = true
			}
		case keyEnter:
			clearMenu(total)
			c := choices[selected]
			fmt.Fprintf(os.Stdout, "%s %s: %s %s\n", green("✓"), title, bold(c.label), dim("— "+c.desc))
			return selected, nil
		case keyCancel:
			clearMenu(total)
			return -1, errCancelled
		}
		if changed {
			redrawMenu(title, choices, selected, total)
		}
	}
}

func renderMenu(title string, choices []selectChoice, selected int) {
	fmt.Fprintln(os.Stdout, bold(cyan(title+":")))
	for i, c := range choices {
		fmt.Fprintln(os.Stdout, menuLine(c, i == selected))
	}
	fmt.Fprintln(os.Stdout, dim("  ↑ ↓ move · Enter select · 1-4 jump · Esc cancel"))
}

// redrawMenu moves back to the title line and draws the menu again
// over the same lines.
func redrawMenu(title string, choices []selectChoice, selected int, total int) {
	fmt.Fprintf(os.Stdout, "\x1b[%dA", total)
	renderMenu(title, choices, selected)
}

// clearMenu erases the menu block, leaving the cursor where the
// confirmation line goes.
func clearMenu(total int) {
	fmt.Fprintf(os.Stdout, "\x1b[%dA", total)
	for i := 0; i < total; i++ {
		fmt.Fprint(os.Stdout, "\r\x1b[K")
		if i < total-1 {
			fmt.Fprint(os.Stdout, "\n")
		}
	}
	fmt.Fprint(os.Stdout, "\r")
}

// menuLine renders one row: the selected row gets a green ❯ and a
// bold label in its accent color, the others get their accent
// color without the bold.
func menuLine(c selectChoice, selected bool) string {
	if selected {
		return "  " + green("❯ ") + bold(c.accent(c.label)) + dim(" — "+c.desc)
	}
	return "    " + c.accent(c.label) + dim(" — "+c.desc)
}
