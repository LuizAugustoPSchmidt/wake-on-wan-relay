package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"slices"
)

func main() {
	if slices.Contains(os.Args, "--version") {
		log.Println("wake-on-wan-relay/key-gen v0.0.1")
		return
	}
	run()
}

func run() {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	fmt.Println("Public: ", hex.EncodeToString(pub))
	fmt.Println("Private:", hex.EncodeToString(priv))
}
