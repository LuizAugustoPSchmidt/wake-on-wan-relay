package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func main() {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	fmt.Println("Public: ", hex.EncodeToString(pub))
	fmt.Println("Private:", hex.EncodeToString(priv))
}
