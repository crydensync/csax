package main

import (
	"bufio"
	"os"
	"strings"
)

// loadEnvFile reads a .env-style file and sets any variable not
// already present in the real environment. Real env vars always win
// — this only fills gaps, matching how most .env loaders behave.
// No external dependency; the format we need (KEY=value, # comments,
// blank lines) is small enough not to justify pulling one in.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env file — fine, real env vars may already be set
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if _, alreadySet := os.LookupEnv(key); !alreadySet {
			os.Setenv(key, val)
		}
	}
}
