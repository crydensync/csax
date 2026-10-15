package main

import (
	"fmt"
	"time"
)

// withSpinner runs fn while showing a spinner, clearing it afterward
// regardless of outcome. Only spins on a real terminal with colors
// enabled — a spinner in piped/scripted output is just noise (and
// worse, corrupts anything parsing that output line by line), same
// reasoning as colorsEnabled gating everywhere else in csax.
//
// Used for anything that makes a real network/DB round trip — the
// AI provider calls in particular took long enough during testing
// that a silent terminal looked hung, not just slow.
func withSpinner(label string, fn func() error) error {
	if !colorsEnabled {
		return fn()
	}

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan struct{})
	go func() {
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Printf("\r%s %s", dim(frames[i%len(frames)]), label)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	err := fn()
	close(done)
	// \r + enough spaces to overwrite the longest spinner line, then
	// \r again to put the cursor back at column 0 for whatever prints
	// next — leaves no leftover spinner text behind.
	fmt.Printf("\r%s\r", spacesLen(len(label)+4))
	return err
}

func spacesLen(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
