package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type output struct {
	GeneratedAtUTC            string `json:"generated_at_utc"`
	Algorithm                 string `json:"algorithm"`
	PrivateKeyFormat          string `json:"private_key_format"`
	PublicKeyFormat           string `json:"public_key_format"`
	RemoteCatalogPrivateKey   string `json:"REMOTE_CATALOG_PRIVATE_KEY"`
	RemoteCatalogPublicKey    string `json:"REMOTE_CATALOG_PUBLIC_KEY"`
	PrivateKeyLengthBytes     int    `json:"private_key_length_bytes"`
	PublicKeyLengthBytes      int    `json:"public_key_length_bytes"`
	SecretsManagerPastePrompt string `json:"secrets_manager_paste_prompt"`
}

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate ed25519 keypair: %v\n", err)
		os.Exit(1)
	}

	result := output{
		GeneratedAtUTC:            time.Now().UTC().Format(time.RFC3339),
		Algorithm:                 "ed25519",
		PrivateKeyFormat:          "base64(raw 64-byte ed25519 private key)",
		PublicKeyFormat:           "base64(raw 32-byte ed25519 public key)",
		RemoteCatalogPrivateKey:   base64.StdEncoding.EncodeToString(priv),
		RemoteCatalogPublicKey:    base64.StdEncoding.EncodeToString(pub),
		PrivateKeyLengthBytes:     len(priv),
		PublicKeyLengthBytes:      len(pub),
		SecretsManagerPastePrompt: "Paste these two keys into your secret JSON as REMOTE_CATALOG_PRIVATE_KEY and REMOTE_CATALOG_PUBLIC_KEY.",
	}

	blob, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal output: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(blob))
}
