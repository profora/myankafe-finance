package ids

import (
  "crypto/rand"
  "encoding/binary"
  "fmt"
  "strings"
  "sync"
  "time"
)

var mu sync.Mutex
var lastMS int64
var seq uint16

func UUIDv7() (string, error) {
  var b [16]byte
  now := time.Now().UnixMilli()
  mu.Lock()
  if now == lastMS { seq++ } else { lastMS = now; seq = 0 }
  s := seq
  mu.Unlock()
  binary.BigEndian.PutUint64(b[:8], uint64(now)<<16|uint64(s))
  if _, err := rand.Read(b[8:]); err != nil { return "", err }
  b[6] = (b[6] & 0x0f) | 0x70
  b[8] = (b[8] & 0x3f) | 0x80
  return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
    binary.BigEndian.Uint32(b[0:4]),
    binary.BigEndian.Uint16(b[4:6]),
    binary.BigEndian.Uint16(b[6:8]),
    binary.BigEndian.Uint16(b[8:10]),
    b[10:16]), nil
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func ULID() (string, error) {
  var raw [16]byte
  ms := uint64(time.Now().UnixMilli())
  raw[0]=byte(ms>>40); raw[1]=byte(ms>>32); raw[2]=byte(ms>>24); raw[3]=byte(ms>>16); raw[4]=byte(ms>>8); raw[5]=byte(ms)
  if _, err := rand.Read(raw[6:]); err != nil { return "", err }
  var out strings.Builder
  out.Grow(26)
  acc := uint64(0); bits := 0
  for _, v := range raw[:] {
    acc = (acc<<8)|uint64(v); bits += 8
    for bits >= 5 {
      bits -= 5
      out.WriteByte(crockford[(acc>>bits)&31])
    }
  }
  if bits > 0 { out.WriteByte(crockford[(acc<<(5-bits))&31]) }
  s := out.String()
  if len(s) < 26 { s += strings.Repeat("0", 26-len(s)) }
  return s[:26], nil
}
