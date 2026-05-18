package database

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// GenerateUUIDv7 produces an RFC 9562 UUIDv7 string.
// Layout: 48-bit unix_ms | 4-bit version(7) | 12-bit rand_a | 2-bit variant(10) | 62-bit rand_b
func GenerateUUIDv7() string {
	var b [16]byte

	// 48-bit millisecond timestamp
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint16(b[0:2], uint16(ms>>32))
	binary.BigEndian.PutUint32(b[2:6], uint32(ms))

	// Fill remaining 10 bytes with crypto random
	if _, err := rand.Read(b[6:]); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}

	// Set version 7 (bits 48-51)
	b[6] = (b[6] & 0x0F) | 0x70

	// Set variant 10 (bits 64-65)
	b[8] = (b[8] & 0x3F) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16],
	)
}
