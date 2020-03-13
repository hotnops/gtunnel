package common

import (
	pb "gTunnel/gTunnel"
	"io"
	"log"
	"net"
)

// A structure to represent a connection
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

func NewConnection(tcpConn net.Conn) *Connection {
	c := new(Connection)
	c.TCPConn = tcpConn
	c.Status = 0
	c.Connected = make(chan bool)
	c.Kill = make(chan bool)

	return c
}

func (c *Connection) SetStream(s ByteStream) {
	c.byteStream = s
}

func (c *Connection) GetStream() ByteStream {
	return c.byteStream
}

func (c *Connection) Close() {
	log.Printf("Closing connection")
	c.TCPConn.Close()
	c.Kill <- true
}

func (c *Connection) handleIngressData() {

	inputChan := make(chan *pb.BytesMessage)

	go func(s ByteStream) {
		for {
			message, _ := s.Recv()
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
			if len(bytesMessage.Content) == 0 {
				log.Printf("TCP connection closed on other side")
				c.remoteClose = true
				c.TCPConn.Close()
				inputChan = nil
				break
			} else {
				bytesSent, err := c.TCPConn.Write(bytesMessage.Content)
				if err != nil {
					log.Printf("Failed to send data to tcp connection: %v", err)
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
	log.Printf("handleIngressData done")
}

func (c *Connection) SendCloseMessage() {
	c.TCPConn.Close()
	closeMessage := new(pb.BytesMessage)
	closeMessage.Content = make([]byte, 0)
	c.byteStream.Send(closeMessage)
}

func (c *Connection) handleEgressData() {
	inputChan := make(chan []byte, 4096)

	go func(t net.Conn) {
		for {
			bytes := make([]byte, 4096)
			bytesRead, err := t.Read(bytes)
			if err != nil {
				if err == io.EOF {
					log.Printf("Connection closed locally")
				} else {
					log.Printf("Error reading tcp data: %v", err)
				}
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
	log.Printf("Handle egressData done")
}

func (c *Connection) Start() {
	log.Printf("Starting connection")
	go c.handleIngressData()
	go c.handleEgressData()
}
