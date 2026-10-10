// Package uuid provides a minimal, allocation-conscious UUID v4 generator
// built directly on crypto/rand. It exists so the SDK keeps zero external
// dependencies.
package uuid

import "crypto/rand"

const hexDigits = "0123456789abcdef"

// NewV4 returns a random (version 4) UUID in the canonical
// 8-4-4-4-12 hexadecimal form, e.g.
// "550e8400-e29b-41d4-a716-446655440000".
//
// It panics only if the system cryptographic random source is broken, which
// on any supported platform means the runtime is already unusable.
func NewV4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("rollypay/uuid: crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return format(b)
}

func format(b [16]byte) string {
	var buf [36]byte
	pos := 0
	for i, v := range b {
		switch i {
		case 4, 6, 8, 10:
			buf[pos] = '-'
			pos++
		}
		buf[pos] = hexDigits[v>>4]
		buf[pos+1] = hexDigits[v&0x0f]
		pos += 2
	}
	return string(buf[:])
}
