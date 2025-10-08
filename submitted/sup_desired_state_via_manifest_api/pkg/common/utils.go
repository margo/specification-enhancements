package common

import (
	"crypto/sha256"
	"fmt"
)

func CalculateDigest(descriptor []byte) string {
	sum := sha256.Sum256(descriptor)
	return fmt.Sprintf("sha256:%x", sum)
}
