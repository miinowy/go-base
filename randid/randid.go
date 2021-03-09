package randid

import (
	"encoding/base64"
	"encoding/binary"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/miinowy/go-base"
)

const (
	seqInc = 673
	seqMax = 0xFFFFFF
)

var (
	seq    uint32
	marker []byte
)

func init() {
	seq = uint32(time.Now().UnixNano()) & seqMax
	marker = make([]byte, 6)
	pid := os.Getpid()
	ip := base.GetIP()
	binary.LittleEndian.PutUint16(marker, uint16(pid))
	if ip != nil && 4 <= len(ip) {
		copy(marker[2:], ip[len(ip)-4:])
	}
}

// Marker return the marker that is used
func Marker() []byte {
	return marker
}

// SetMarker manually
func SetMarker(buf []byte) {
	marker = buf
}

// New return a Rand ID
//
// Rand ID is generate by Marker combined Timestamp and Sequence
//
// Marker:    The marker consists of ip and pid by default
// Timestamp: Only the lowest 24 bits of timestamp are used,
//            So a loop will be generated after 2^24 seconds(about 200 days)
// Sequence:  The sequence will not repeat until all numbers(2^24)
//            are used up, follows three rules bellow:
//              1. The sequence is incremented by 673 each time;
//              2. The sequence will overflow after 2^24;
//              3. (2^24+1) % 673 == 0;
func New() string {
	src := make([]byte, 12)
	dst := make([]byte, 16)
	binary.LittleEndian.PutUint32(dst, uint32(time.Now().Unix()))
	binary.LittleEndian.PutUint32(dst[3:], atomic.AddUint32(&seq, seqInc))
	copy(src[:6], marker)
	copy(src[6:], dst)
	for i := 0; i < 6; i++ {
		src[i*2], src[6+i] = src[6+i], src[i*2]
		src[i*2] = src[i*2] ^ src[9]
	}
	base64.URLEncoding.Encode(dst, src)
	return string(dst)
}

// Decode return IP and Time from Rand ID
func Decode(id string) (net.IP, time.Time) {
	var timestamp uint64
	src := []byte(id)
	dst := make([]byte, 12)
	if len(src) != 16 {
		return nil, time.Time{}
	}
	if _, err := base64.URLEncoding.Decode(dst, src); err != nil {
		return nil, time.Time{}
	}
	for i := 5; 0 <= i; i-- {
		dst[i*2] = dst[i*2] ^ dst[9]
		dst[i*2], dst[6+i] = dst[6+i], dst[i*2]
	}
	dst[9] = 0
	timestamp = uint64(binary.LittleEndian.Uint32(dst[6:]))
	timestamp |= uint64(time.Now().Unix()) & 0xFF000000
	return dst[2:6], time.Unix(int64(timestamp), 0)
}
