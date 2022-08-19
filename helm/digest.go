package helm

import (
	"crypto"
	"encoding/hex"
	"io"
	"os"
)

// DigestFile calculates a SHA256 hash (like Docker) for a given file.
//
// It takes the path to the archive file, and returns a string representation of
// the SHA256 sum.
//
// The intended use of this function is to generate a sum of a chart TGZ file.
func DigestFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return Digest(f)
}

// Digest hashes a reader and returns a SHA256 digest.
//
// Helm uses SHA256 as its default hash for all non-cryptographic applications.
func Digest(in io.Reader) (string, error) {
	hash := crypto.SHA256.New()
	if _, err := io.Copy(hash, in); err != nil {
		return "", nil
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
