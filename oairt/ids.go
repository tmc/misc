package oairt

import (
	"crypto/rand"
	"encoding/binary"
)

// idCharset is the alphabet used for event IDs. Excludes 0/O/I/l to avoid
// visual confusion in logs.
const idCharset = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// NewEventID returns a new event identifier prefixed with prefix and padded
// to 21 characters total. IDs are drawn from a cryptographic source.
//
// Example: NewEventID("evt_") returns a string like "evt_8aBcD2eFgHJk3MnPqRsT".
func NewEventID(prefix string) string {
	bodyLen := 21 - len(prefix)
	if bodyLen <= 0 {
		return prefix
	}
	body := make([]byte, bodyLen)
	var buf [8]byte
	for i := 0; i < bodyLen; i++ {
		if _, err := rand.Read(buf[:]); err != nil {
			// crypto/rand.Read should never fail on supported platforms; if
			// it does, callers prefer a panic to a non-unique fallback.
			panic("oairt: crypto/rand failed: " + err.Error())
		}
		idx := binary.BigEndian.Uint64(buf[:]) % uint64(len(idCharset))
		body[i] = idCharset[idx]
	}
	return prefix + string(body)
}
