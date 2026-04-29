package generator

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

const (
	tokenLength = 32
	baseOfCode  = 1_000_000
)

func GenerateToken() (raw string, hash string, err error) {
	b := make([]byte, tokenLength)
	_, err = rand.Read(b)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw = hex.EncodeToString(b)
	hash = HashToken(raw)
	return raw, hash, nil
}

func GenerateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(baseOfCode))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func HashToken(raw string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
}
