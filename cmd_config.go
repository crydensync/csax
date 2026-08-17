package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func cmdConfigInit(args []string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Database connection string: ")
	dbURL, _ := reader.ReadString('\n')
	dbURL = strings.TrimSpace(dbURL)
	if dbURL == "" {
		fmt.Println("A database connection string is required.")
		os.Exit(1)
	}

	fmt.Print("JWT secret (leave blank to generate one): ")
	secret, _ := reader.ReadString('\n')
	secret = strings.TrimSpace(secret)
	if secret == "" {
		secret = generateSecret()
		fmt.Println("Generated a new JWT secret.")
	}

	content := fmt.Sprintf("DATABASE_URL=%s\nJWT_SECRET=%s\nMIGRATIONS_DIR=./migrations\n", dbURL, secret)

	if _, err := os.Stat(".env"); err == nil {
		fmt.Print(".env already exists — overwrite? [y/N]: ")
		confirm, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(confirm)) != "y" {
			fmt.Println("Aborted — .env left unchanged.")
			return
		}
	}

	if err := os.WriteFile(".env", []byte(content), 0600); err != nil {
		fmt.Printf("failed to write .env: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✔ Wrote .env")
}

func generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}
