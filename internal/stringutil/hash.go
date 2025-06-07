package stringutil

import (
	"crypto/sha256"
	"math/big"
)

// Base58 alphabet used by Bitcoin
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

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
	// Convert bytes to big integer
	intBytes := big.NewInt(0)
	intBytes.SetBytes(input)
	
	// Convert to base58
	var result []byte
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
	result := big.NewInt(0)
	base := big.NewInt(58)
	
	// Build character to value map
	charMap := make(map[rune]int64)
	for i, c := range base58Alphabet {
		charMap[c] = int64(i)
	}
	
	// Process each character
	for _, c := range input {
		val, ok := charMap[c]
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
	
	// Prepend zeros
	zeros := make([]byte, numZeros)
	return append(zeros, decoded...), nil
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