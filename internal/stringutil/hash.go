package stringutil

import (
	"crypto/sha256"
	"math/big"
)

// Base58 alphabet used by Bitcoin (excludes 0, O, l, I to avoid ambiguity)
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Pre-computed character to value map for Base58Decode
var base58CharMap = func() map[rune]int64 {
	m := make(map[rune]int64, len(base58Alphabet))
	for i, c := range base58Alphabet {
		m[c] = int64(i)
	}
	return m
}()

// Base58EncodeSHA256 generates a SHA-256 hash of the input and encodes it as base58.
// This is useful for creating deterministic, URL-safe identifiers.
func Base58EncodeSHA256(input string) string {
	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(input))
	
	// Convert hash to base58
	return Base58Encode(hash[:])
}

// Base58Encode encodes a byte slice to base58 string.
// Uses the Bitcoin alphabet which excludes ambiguous characters (0, O, l, I).
func Base58Encode(input []byte) string {
	if len(input) == 0 {
		return ""
	}

	// Convert bytes to big integer
	intBytes := big.NewInt(0)
	intBytes.SetBytes(input)
	
	// Pre-allocate result with estimated capacity
	// Base58 encoding expands by approximately 138%
	estimatedLen := len(input)*138/100 + 1
	result := make([]byte, 0, estimatedLen)
	
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := &big.Int{}
	
	for intBytes.Cmp(zero) > 0 {
		intBytes.DivMod(intBytes, base, mod)
		result = append(result, base58Alphabet[mod.Int64()])
	}
	
	// Handle leading zeros in input
	for _, b := range input {
		if b != 0 {
			break
		}
		result = append(result, base58Alphabet[0])
	}
	
	// Reverse the result
	return reverseString(string(result))
}

// Base58Decode decodes a base58 string to bytes.
// Returns an error if the string contains invalid characters.
func Base58Decode(input string) ([]byte, error) {
	if len(input) == 0 {
		return []byte{}, nil
	}

	result := big.NewInt(0)
	base := big.NewInt(58)
	
	// Process each character using pre-computed map
	for _, c := range input {
		val, ok := base58CharMap[c]
		if !ok {
			return nil, &base58Error{char: c}
		}
		
		result.Mul(result, base)
		result.Add(result, big.NewInt(val))
	}
	
	// Convert to bytes
	decoded := result.Bytes()
	
	// Handle leading '1's (zeros)
	var numZeros int
	for _, c := range input {
		if c != '1' {
			break
		}
		numZeros++
	}
	
	// Prepend zeros if needed
	if numZeros > 0 {
		zeros := make([]byte, numZeros)
		return append(zeros, decoded...), nil
	}
	
	return decoded, nil
}

// reverseString reverses a string efficiently
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// base58Error represents an invalid base58 character error
type base58Error struct {
	char rune
}

func (e *base58Error) Error() string {
	return "invalid base58 character: " + string(e.char)
}