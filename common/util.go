package common

import (
	"encoding/binary"
	"math/rand"
	"net"
	"time"
)

// ClientIDSize is the constant used for the string size
// of a generated client ID
const ClientIDSize = 8

// TunnelIDSize is the constant used for the string size
// of a generated tunnel ID
const TunnelIDSize = 8

// GenerateString will generate a random string of length
// Credit to: https://www.calhoun.io/creating-random-strings-in-go/
func GenerateString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// Int32ToIP converts a uint32 to a net.IP
func Int32ToIP(i uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, i)
	return ip
}

// IpToInt32 converts a net.IP to a uint32
// Credit to https://gist.github.com/ammario/ipint.go
func IpToInt32(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}
