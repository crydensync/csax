package main

import "os"

// Minimal ANSI helpers — no dependency, matches csax's existing
// stdlib-only philosophy. Colors are skipped automatically when
// output isn't a real terminal (e.g. piped to a file or another
// program), so scripting against csax output never sees raw escape
// codes mixed into the data.

var colorsEnabled = isTerminal()

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func green(s string) string {
	if !colorsEnabled {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func red(s string) string {
	if !colorsEnabled {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func yellow(s string) string {
	if !colorsEnabled {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func dim(s string) string {
	if !colorsEnabled {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}
