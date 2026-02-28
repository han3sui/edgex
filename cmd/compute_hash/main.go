package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func main() {
	password := "admin"
	nonce := "a671c864d8e0978562954ae1e7beecda"
	combined := password + nonce
	hash := sha256.Sum256([]byte(combined))
	hashString := hex.EncodeToString(hash[:])
	fmt.Println(hashString)
}
