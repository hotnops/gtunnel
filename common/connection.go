package common

import (
	pb "gTunnel/gTunnel"
	"net"
)

// A structure to handle the TCP connection
// and map them to the gRPC byte stream.
type Connection struct {
	Id          int32
	TCPConn     net.Conn
	Kill        chan bool
	Status      int32
	Connected   chan bool
	ingressData chan *pb.BytesMessage
	egressData  chan *pb.BytesMessage
	byteStream  ByteStream
	bytesTx     uint64
	bytesRx     uint64
	remoteClose bool
}

// NewConnection is a constructor function for Connection.
func NewConnection(tcpConn net.Conn) *Connection {
	c := new(Connection)
	c.TCPConn = tcpConn
	c.Status = 0
	c.Connected = make(chan bool)
	c.Kill = make(chan bool)

	return c
}

// SetStream will set the byteStream for a connection
func (c *Connection) SetStream(s ByteStream) {
	c.byteStream = s
}

// GetStream will return the byteStream for a connection
func (c *Connection) GetStream() ByteStream {
	return c.byteStream
}

// Close will close a TCP connection and close the
// Kill channel.
func (c *Connection) Close() {
	c.TCPConn.Close()
	if c.Status != ConnectionStatusClosed {
		close(c.Kill)
		c.Status = ConnectionStatusClosed
	}
}

// handleIngressData will handle all incoming messages
// on the gRPC byte stream and send them to the locally
// connected socket.
func (c *Connection) handleIngressData() {

	inputChan := make(chan *pb.BytesMessage)

	go func(s ByteStream) {
		for {
			message, err := s.Recv()
			if err != nil {
				c.Close()
				return
			}
			inputChan <- message
		}
	}(c.byteStream)

	for {
		select {
		case bytesMessage, ok := <-inputChan:
			if !ok {
				inputChan = nil
				break
			}
			if bytesMessage == nil {
				c.TCPConn.Close()
				inputChan = nil
				break
			} else if len(bytesMessage.Content) == 0 {
				c.remoteClose = true
				c.TCPConn.Close()
				inputChan = nil
				break
			} else {
				bytesSent, err := c.TCPConn.Write(bytesMessage.Content)
				if err != nil {
					c.SendCloseMessage()
					inputChan = nil
					break
				} else {
					c.bytesTx += uint64(bytesSent)
				}
			}
		}
		if inputChan == nil {
			break
		}
	}
}

// SendCloseMessage will send a zero sized
// message to the remote endpoint, indicating
// that a TCP connection has been closed locally.
func (c *Connection) SendCloseMessage() {
	c.TCPConn.Close()
	closeMessage := new(pb.BytesMessage)
	closeMessage.Content = make([]byte, 0)
	c.byteStream.Send(closeMessage)
}

// handleEgressData will listen on the locally
// connected TCP socket and send the data over the gRPC stream.
func (c *Connection) handleEgressData() {
	inputChan := make(chan []byte, 4096)

	go func(t net.Conn) {
		for {
			bytes := make([]byte, 4096)
			bytesRead, err := t.Read(bytes)
			if err != nil {
				if !c.remoteClose {
					bytes = make([]byte, 4096)
				}
			}
			inputChan <- bytes[:bytesRead]
			if err != nil {
				break
			}
		}
	}(c.TCPConn)

	for {
		select {
		case bytes, ok := <-inputChan:
			if !ok {
				inputChan = nil
				break
			}
			message := new(pb.BytesMessage)
			message.Content = bytes

			c.byteStream.Send(message)
			if len(message.Content) == 0 {
				inputChan = nil
				break
			}
		}
		if inputChan == nil {
			break
		}
	}
}

// Start will start two goroutines for handling the TCP socket
// and the gRPC stream.
func (c *Connection) Start() {
	go c.handleIngressData()
	go c.handleEgressData()
}
