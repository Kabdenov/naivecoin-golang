// utils provides tools for hashing, crypto tasks
package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// signature is a struct to hold R and S values for a ecdsa signature
type signature struct {
	R, S *big.Int
}

// Hash computes a SHA-256 hash for a given object
// https://blog.8bitzen.com/posts/22-08-2019-how-to-hash-a-struct-in-go
func Hash(o interface{}) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", o)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

var hexDict = map[string]string{
	"0": "0000",
	"1": "0001",
	"2": "0010",
	"3": "0011",
	"4": "0100",
	"5": "0101",
	"6": "0110",
	"7": "0111",
	"8": "1000",
	"9": "1001",
	"a": "1010",
	"b": "1011",
	"c": "1100",
	"d": "1101",
	"e": "1110",
	"f": "1111",
}

// HexToBin converts hex string to a string of ones and zeros (binary representation)
func HexToBin(hex string) (string, error) {
	hex = strings.ToLower(hex)
	var result string
	for n := 0; n < len(hex); n++ {
		result += hexDict[string(hex[n])]
	}
	return result, nil
}

// IsHex checks if a given string has only hex characters
func IsHex(s string) bool {
	var hexTest = regexp.MustCompile(`^[0-9a-fA-F]+$`)
	return hexTest.MatchString(s)
}

// GetPublicKey computes hex encoded public key from a given hex encoded private key
func GetPublicKey(privateKey string) (publicKey string) {
	pk := hexToPrivateKey(privateKey)
	return fmt.Sprintf("%x", elliptic.Marshal(secp256k1.S256(), pk.PublicKey.X, pk.PublicKey.Y))
}

// hexToPrivateKey converts hex encoded private key to a PrivateKey struct
func hexToPrivateKey(privateKey string) *ecdsa.PrivateKey {
	pk := new(ecdsa.PrivateKey)
	pk.D, _ = new(big.Int).SetString(privateKey, 16)
	pk.PublicKey.Curve = secp256k1.S256()
	pk.PublicKey.X, pk.PublicKey.Y = pk.PublicKey.Curve.ScalarBaseMult(pk.D.Bytes())
	return pk
}

// hexToPublicKey converts hex encoded public key to a PublicKey struct
func hexToPublicKey(publicKey string) *ecdsa.PublicKey {
	pk := new(ecdsa.PublicKey)
	publicKeyBytes, _ := hex.DecodeString(publicKey)
	x, y := elliptic.Unmarshal(secp256k1.S256(), publicKeyBytes)
	pk.X = x
	pk.Y = y
	pk.Curve = secp256k1.S256()
	return pk
}

// GetSignature returns a signature for a given hash, using provided private key
func GetSignature(hash string, privateKey string) string {
	hashBytes, _ := hex.DecodeString(hash)
	key := hexToPrivateKey(privateKey)

	var sig signature = signature{}
	r, s, _ := ecdsa.Sign(rand.Reader, key, hashBytes)
	sig.R = r
	sig.S = s

	marshaled, _ := asn1.Marshal(sig)
	return fmt.Sprintf("%x", marshaled)
}

// VerifySignature verifies a signature for a given hash, using provided public key
func VerifySignature(hash string, sig string, publicKeyHex string) bool {
	publicKey := hexToPublicKey(publicKeyHex)
	hashBytes, _ := hex.DecodeString(hash)

	sigBytes, _ := hex.DecodeString(sig)

	var esig signature
	// invalid signature will be unmarshaled incorrectly and cause a memory address error when verified
	// check for error to prevent this from happening
	_, err := asn1.Unmarshal(sigBytes, &esig)
	if err == nil {
		return ecdsa.Verify(publicKey, hashBytes, esig.R, esig.S)
	}
	return false
}

func Base58Encode(str string) string {
	bytes, _ := hex.DecodeString(str)
	return base58.Encode(bytes)
}

func Base58Decode(base58str string) string {
	bytes := base58.Decode(base58str)
	return hex.EncodeToString(bytes)
}
