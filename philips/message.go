package philips

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// SessionID defines the starting ID of a "session". For every command
// sent, the session ID needs to be incremented by one. You get the
// starting ID by posting to /sys/dev/sync and storing the response
//
// It's more or less the CoAP Message ID, but they botched it by setting
// a CoAP MID of 1 on every message, so we get this magic instead.
type SessionID uint32

const (
	checksumLen = 32
	magicWord   = "JiangPan"
)

var (
	rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// ParseID parses a sequence of bytes into a SessionID
func ParseID(data []byte) SessionID {
	if len(data) > 8 {
		data = data[:8]
	}
	s, _ := strconv.ParseUint(string(data), 16, 32)
	return SessionID(s)
}

// NewID constructs a new valid SessionID
func NewID() SessionID {
	return SessionID(rnd.Uint32())
}

// Hex returns the hex representation of our SessionID
func (id SessionID) Hex() string {
	return fmt.Sprintf("%08X", id)
}

func (id SessionID) keyIV() (key, iv []byte) {
	keyAndIV := md5.Sum([]byte(magicWord + id.Hex()))
	// The key and IV are "stretched" from 8 bytes to 16 by hex encoding
	// the two halves
	key = []byte(strings.ToUpper(hex.EncodeToString(keyAndIV[0:8])))
	iv = []byte(strings.ToUpper(hex.EncodeToString(keyAndIV[8:])))
	return
}

// Decrypt returns the plaintext for a message using AES-128 in CBC
// with a key/IV derived from the SessionID
func (id SessionID) Decrypt(data []byte) ([]byte, error) {
	key, iv := id.keyIV()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	cbc := cipher.NewCBCDecrypter(block, iv)
	d := make([]byte, len(data))
	cbc.CryptBlocks(d, data)
	return d, nil
}

// Encrypt returns the ciphertext for a message using AES-128 in CBC
// with a key/IV derived from the SessionID
func (id SessionID) Encrypt(data []byte) ([]byte, error) {
	key, iv := id.keyIV()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	cbc := cipher.NewCBCEncrypter(block, iv)
	d := make([]byte, len(data))
	cbc.CryptBlocks(d, data)
	return d, nil
}

// DecodeMessage returns the plaintext of a received message
// `msg` is the message as received (i.e. the hex-encoded string)
func DecodeMessage(msg []byte) ([]byte, error) {
	sess := ParseID(msg)
	data, err := hex.DecodeString(string(msg))
	if err != nil {
		return nil, fmt.Errorf("error decoding hex: %w", err)
	}
	if len(data) < 4+checksumLen {
		return nil, fmt.Errorf("too few bytes")
	}

	data = data[4:]
	// Ignore the checksum, ethernet and UDP already have checksums and since
	// it's just a plain hash, not an HMAC, verifying it doesn't help us
	data = data[:len(data)-checksumLen]

	out, err := sess.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt: %w", err)
	}

	// Strip/ignore the padding
	// Messages are padded to 16 bytes, and the padding character is also
	// the amount of padding included. For example, If the response contains
	// 15 bytes of padding it'll be padded with 0x0f
	for len(out) > 0 {
		// There's an off-by-one in how they pad messages. If a message is 16
		// bytes, there's no need for any padding, but sometimes you get a
		// second block that's just 16 bytes of 0x10
		if out[len(out)-1] < 1 || out[len(out)-1] > 16 {
			break
		}
		out = out[:len(out)-1]
	}
	return out, nil
}

// EncodeMessage returns the ciphertext of a message. This will generally be
// a JSON encoded request
func EncodeMessage(sess SessionID, msg []byte) ([]byte, error) {
	if len(msg) == 0 {
		return nil, fmt.Errorf("too few bytes")
	}

	padding := (16 - (len(msg) % 16)) % 16
	for i := 0; i < padding; i++ {
		msg = append(msg, byte(padding))
	}
	out, err := sess.Encrypt(msg)
	if err != nil {
		return nil, fmt.Errorf("unable to encrypt: %w", err)
	}
	outMsg := sess.Hex() + strings.ToUpper(hex.EncodeToString(out))
	// For some reason we need to append the SHA-256 hash of the ciphertext to
	// the message. This seems pretty pointless since ethernet and UDP already
	// have checksumming, and hashing the encrypted message is not a security
	// feature since anyone can do that. It's also just a hash, not an HMAC.
	shaSum := sha256.Sum256([]byte(outMsg))
	outMsg += strings.ToUpper(hex.EncodeToString(shaSum[:]))
	return []byte(outMsg), nil
}
