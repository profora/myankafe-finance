package ids

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"
	"time"
)

var uuidMu sync.Mutex
var lastMS int64
var seq uint16

func UUIDv7() (string, error) {
	var b [16]byte
	now := time.Now().UnixMilli()

	uuidMu.Lock()
	if now == lastMS {
		seq++
	} else {
		lastMS = now
		seq = 0
	}
	s := seq
	uuidMu.Unlock()

	binary.BigEndian.PutUint64(b[:8], uint64(now)<<16|uint64(s))
	if _, err := rand.Read(b[8:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16],
	), nil
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func ULID() (string, error) {
	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}

	var out [26]byte
	ms := uint64(time.Now().UnixMilli())

	// ULID timestamp: 48 bits encoded into the first 10 Crockford Base32 chars.
	for i := 9; i >= 0; i-- {
		out[i] = crockford[ms&31]
		ms >>= 5
	}

	// ULID randomness: 80 bits encoded into the remaining 16 chars.
	n := new(big.Int).SetBytes(entropy[:])
	mask := big.NewInt(31)
	for i := 25; i >= 10; i-- {
		idx := new(big.Int).And(n, mask).Int64()
		out[i] = crockford[idx]
		n.Rsh(n, 5)
	}

	return string(out[:]), nil
}
