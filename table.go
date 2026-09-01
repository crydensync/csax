package main

import (
	"fmt"
	"strings"
)

// maxCellWidth caps how wide any single column gets — a full UUID or
// long email would otherwise blow up every row's width. Truncated
// values end in "…" so it's visually obvious they're cut, not just
// short data.
const maxCellWidth = 36

// printTable renders columns/rows as a real box-drawing table. Falls
// back to plain columns with no box characters when colors are
// disabled (piped output, NO_COLOR, non-TTY) — box-drawing characters
// in a script's captured output are just noise for whatever's
// parsing it, same reasoning as the existing color-disable behavior.
func printTable(columns []string, rows [][]string) {
	if !colorsEnabled {
		printPlainTable(columns, rows)
		return
	}

	cells := make([][]string, len(rows)+1)
	cells[0] = truncateRow(columns)
	for i, row := range rows {
		cells[i+1] = truncateRow(row)
	}
	widths := columnWidths(cells)

	printBorder(widths, "┌", "┬", "┐")
	printRow(cells[0], widths, true)
	printBorder(widths, "├", "┼", "┤")
	for i := 1; i < len(cells); i++ {
		printRow(cells[i], widths, false)
	}
	printBorder(widths, "└", "┴", "┘")
}

func printPlainTable(columns []string, rows [][]string) {
	println_ := func(cells []string) {
		printfLine(strings.Join(cells, " | "))
	}
	println_(columns)
	for _, row := range rows {
		println_(row)
	}
}

func printfLine(s string) {
	// Small indirection so this file has exactly one place that
	// writes plain lines — kept separate from fmt.Println calls
	// elsewhere so a future output-destination change (e.g. writing
	// to a log file too) only touches one spot.
	fmt.Println(s)
}

func truncateRow(row []string) []string {
	out := make([]string, len(row))
	for i, v := range row {
		out[i] = truncateCell(v)
	}
	return out
}

func truncateCell(v string) string {
	if len(v) <= maxCellWidth {
		return v
	}
	return v[:maxCellWidth-1] + "…"
}

func columnWidths(cells [][]string) []int {
	widths := make([]int, len(cells[0]))
	for _, row := range cells {
		for i, v := range row {
			if len([]rune(v)) > widths[i] {
				widths[i] = len([]rune(v))
			}
		}
	}
	return widths
}

func printBorder(widths []int, left, mid, right string) {
	var b strings.Builder
	b.WriteString(dim(left))
	for i, w := range widths {
		b.WriteString(dim(strings.Repeat("─", w+2)))
		if i < len(widths)-1 {
			b.WriteString(dim(mid))
		}
	}
	b.WriteString(dim(right))
	fmt.Println(b.String())
}

func printRow(cells []string, widths []int, header bool) {
	var b strings.Builder
	b.WriteString(dim("│"))
	for i, v := range cells {
		padded := v + strings.Repeat(" ", widths[i]-len([]rune(v)))
		if header {
			padded = dim("\033[1m" + padded + "\033[22m")
		} else {
			padded = colorizeCell(v, padded)
		}
		b.WriteString(" " + padded + " ")
		b.WriteString(dim("│"))
	}
	fmt.Println(b.String())
}

// colorizeCell gives audit event types a semantic color — the
// specific thing the earlier "any failed logins recently" test made
// obvious was missing: a wall of same-colored text makes it hard to
// spot the one row that matters. Anything else prints unchanged.
func colorizeCell(rawValue, padded string) string {
	switch {
	case strings.Contains(rawValue, "failed"), strings.Contains(rawValue, "reuse_detected"), strings.Contains(rawValue, "locked"):
		return red(padded)
	case strings.Contains(rawValue, "success"), strings.Contains(rawValue, "linked"), strings.Contains(rawValue, "unlocked"):
		return green(padded)
	default:
		return padded
	}
}
