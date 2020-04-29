package common

import (
	"fmt"
	"net"
	"time"

	"github.com/fangdingjun/socks-go"
)

type SocksServer struct {
	listener    net.Listener
	connections []socks.Conn
	servePort   uint32
}

func NewSocksServer(port uint32) *SocksServer {
	s := new(SocksServer)
	s.servePort = port
	s.connections = make([]socks.Conn, 0)
	return s
}

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

func (s *SocksServer) Stop() {
	s.listener.Close()
	for _, conn := range s.connections {
		conn.Close()
	}
}
