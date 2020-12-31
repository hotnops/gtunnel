package common

import (
	"fmt"
	"net"
	"time"

	"github.com/fangdingjun/socks-go"
)

// SocksServer is a structure that handles starting and stopping
// a socks v5 proxy on the gClient.
type SocksServer struct {
	listener    net.Listener
	connections []socks.Conn
	servePort   uint32
}

// NewSocksServer is a constructor for the SocksServer struct.
// It takes in a port as an argument, wich will be the port on
// which the socks server listens.
func NewSocksServer(port uint32) *SocksServer {
	s := new(SocksServer)
	s.servePort = port
	s.connections = make([]socks.Conn, 0)
	return s
}

// Start will start the socks server. Simple enough.
func (s *SocksServer) Start() bool {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.servePort))

	if err != nil {
		return false
	}

	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				break
			}
			d := net.Dialer{Timeout: 10 * time.Second}
			newConn := socks.Conn{Conn: conn, Dial: d.Dial}
			s.connections = append(s.connections, newConn)
			go newConn.Serve()
		}
	}()
	return true
}

// Stop - You'll never guess what this does.
func (s *SocksServer) Stop() {
	s.listener.Close()
	for _, conn := range s.connections {
		conn.Close()
	}
}
