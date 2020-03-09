package common

import (
	"net"
	pb "hotnops/gTunnel/gTunnel"
)

// A structure to represent a connection
type Connection struct {
	TCPConn   net.Conn
	Stream    pb.GTunnel_CreateByteStreamClient
	Kill      chan bool
	Status    int32
	Connected chan bool
}

func NewConnection(tcpConn net.Conn) *Connection {
	c := new(Connection)
	c.TCPConn = tcpConn
	c.Stream = nil
	c.Status = 0
	c.Connected = make(chan bool)
	return c
}
