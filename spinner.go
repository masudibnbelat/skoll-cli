package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// A small loading spinner for the slow, quiet steps (downloads,
// looking up the latest versions). While Gradle builds or adb runs,
// their own output is the visualization, so no spinner is used there.
// When stdout is not a terminal (piped / CI), the spinner stays off
// and plain lines are printed instead.

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinner struct {
	done   chan struct{}
	wg     sync.WaitGroup
	active bool
	start  time.Time
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func startSpinner(msg string) *spinner {
	s := &spinner{done: make(chan struct{}), start: time.Now()}
	if !isTerminal(os.Stdout) {
		return s
	}
	s.active = true
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		i := 0
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-tick.C:
				fmt.Fprintf(os.Stdout, "\r%s %s  %s", cyan(spinnerFrames[i%len(spinnerFrames)]), msg, dim(formatDuration(time.Since(s.start))))
				i++
			}
		}
	}()
	return s
}

// stop clears the spinner line and prints the finished message.
func (s *spinner) stop(msg string) {
	if s.active {
		close(s.done)
		s.wg.Wait()
		fmt.Fprintf(os.Stdout, "\r\x1b[K%s %s\n", green("✓"), msg)
		return
	}
	fmt.Println("== " + msg)
}

// stopFail clears the spinner line and prints a failed message.
func (s *spinner) stopFail(msg string) {
	if s.active {
		close(s.done)
		s.wg.Wait()
		fmt.Fprintf(os.Stdout, "\r\x1b[K%s %s\n", red("✗"), msg)
		return
	}
	fmt.Println("STOP: " + msg)
}
