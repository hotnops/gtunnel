package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"hotnops/gTunnel/common"
	pb "hotnops/gTunnel/gTunnel"

	"google.golang.org/grpc"
	"google.golang.org/grpc/testdata"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "", "The file containing the CA root cert file")
	serverAddr         = flag.String("server_addr", "127.0.0.1:10000", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake")
)

func intToIP(ip uint32) string {
	result := make(net.IP, 4)
	result[3] = byte(ip)
	result[2] = byte(ip >> 8)
	result[1] = byte(ip >> 16)
	result[0] = byte(ip >> 24)
	return result.String()
}

type client struct {
	connections map[int32]common.Connection
	ctrlStream  pb.GTunnel_GetConnectionMessagesClient
	tStream     pb.GTunnel_CreateByteStreamClient
	gClient     pb.GTunnelClient
	gCtx        context.Context
	kill        chan bool
}

func NewClient() *client {
	c := new(client)
	c.connections = make(map[int32]common.Connection)
	c.ctrlStream = nil
	c.tStream = nil
	c.gClient = nil
	return c
}

func (c *client) deleteConnection(connID int32) {
	log.Printf("Deleting connection: %d", connID)
	delete(c.connections, connID)
	bytesMessage := new(pb.BytesMessage)
	bytesMessage.ConnectionID = connID
	c.gClient.AcknowledgeDisconnect(c.gCtx, bytesMessage)
}

// This function will loop reading data from the gTunnel
// and send it to the proper socket.
func (c *client) receiveServerData() {
	for {
		bMessage := new(pb.BytesMessage)
		err := c.tStream.RecvMsg(bMessage)
		if err != nil {
			log.Fatal("Failed to receive tunnel data")
		}
		connection, ok := c.connections[bMessage.ConnectionID]
		if !ok {
			log.Printf("Failed to lookup connection id: %d", bMessage.ConnectionID)
			continue
		}
		if len(bMessage.GetContent()) == 0 {
			log.Printf("Received remote disconnect")
			connection.TCPConn.Close()
			c.deleteConnection(bMessage.ConnectionID)
		}
		_, err = connection.TCPConn.Write(bMessage.GetContent())
		if err != nil {
			log.Printf("Failed to write bytes to connection: %v", err)
		}
	}
}

func (c *client) addConnection(conID int32, conn net.Conn) {
	newConn := common.NewConnection(conn)
	newConn.Stream = c.tStream
	newConn.Kill = make(chan bool)
	c.connections[conID] = *newConn
}

func (c *client) sendDataToServer(connID int32) {
	connection, ok := c.connections[connID]
	if !ok {
		log.Printf("Failed to look up connection for connID : %d", connID)
	}
	conn := connection.TCPConn
	for {
		b := make([]byte, 4096)
		bRead, err := conn.Read(b)
		if err == io.EOF || bRead == 0 {
			log.Printf("Server closed connection")
			break
		}
		bMessage := new(pb.BytesMessage)
		bMessage.ConnectionID = connID
		bMessage.Content = b[:bRead]
		err = c.tStream.SendMsg(bMessage)
		if err != nil {
			log.Printf("Failed to write content to tunnel stream")
		}
	}
}

func (c *client) handleTunnelConnection(message *pb.ConnectionMessage) {
	address := intToIP(message.IpAddress)
	addressString := fmt.Sprintf("%s:%d", address, message.Port)
	log.Printf("Connecting to %s", addressString)
	conn, err := net.Dial("tcp", addressString)
	if err != nil {
		log.Printf("failed to connect to server: %v", err)
		message.ErrStatus = 1
		c.gClient.SendMessageToServer(c.gCtx, message)
		return
	}

	log.Printf("Successfully connected to server")

	//c.gClient.ConfirmConnection()
	// Add the connection to our map
	c.addConnection(message.ConnectionID, conn)
	c.gClient.SendMessageToServer(c.gCtx, message)

	// Start receiving data from the connection to send over the tunnel
	go c.sendDataToServer(message.ConnectionID)
}

func (c *client) receiveControlMessages() {
	for {
		message, err := c.ctrlStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v = %v", c, err)
		}
		log.Printf("received message %d %d %d", message.GetConnectionID(),
			message.GetIpAddress(), message.GetPort())

		if message.Operation == 1 {
			c.handleTunnelConnection(message)
		}
	}
}

func main() {
	flag.Parse()
	var err error
	var cancel context.CancelFunc

	var opts []grpc.DialOption
	if *tls {
		if *caFile == "" {
			*caFile = testdata.Path("ca.pem")
		}
	}
	opts = append(opts, grpc.WithInsecure())

	client := NewClient()
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client.gClient = pb.NewGTunnelClient(conn)
	client.gCtx, cancel = context.WithCancel(context.Background())
	defer cancel()

	empty := pb.Empty{}
	client.ctrlStream, err = client.gClient.GetConnectionMessages(client.gCtx, &empty)
	if err != nil {
		log.Fatalf("GetControlMessage failed: %v : %v", client, err)
	}
	client.tStream, err = client.gClient.CreateByteStream(client.gCtx)
	if err != nil {
		log.Fatalf("CreateByteStream failed: %v : %v", client, err)
	}
	go client.receiveControlMessages()
	go client.receiveServerData()
	<-client.kill
}
