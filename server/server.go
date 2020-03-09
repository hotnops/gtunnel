package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"hotnops/gTunnel/common"
	pb "hotnops/gTunnel/gTunnel"

	"github.com/abiosoft/ishell"
	"google.golang.org/grpc"
)

var (
	tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "", "The TLS cert file")
	keyFile    = flag.String("key_file", "", "The TLS key file")
	jsonDBFile = flag.String("json_db_file", "", "A json file containing a list of features")
	port       = flag.Int("port", 10000, "The server port")
)

type server struct {
	pb.UnimplementedGTunnelServer
	output       chan pb.ConnectionMessage
	connections  map[int32]common.Connection
	messageCount int32
	tStream      pb.GTunnel_CreateByteStreamServer
}

// RPC function that will listen on the output channel and send
// any messages over to the gClient.
func (s *server) GetConnectionMessages(empty *pb.Empty, stream pb.GTunnel_GetConnectionMessagesServer) error {
	log.Println("Client connected...")
	for {
		message := <-s.output
		if err := stream.Send(&message); err != nil {
			log.Fatalf("failed to send: %v", err)
			return err
		}
	}
}

// This is an RPC function the the client will call when it can confirm that
// the server has successfully disconnected.
func (s *server) AcknowledgeDisconnect(ctx context.Context, message *pb.BytesMessage) (*pb.Empty, error) {
	connID := message.GetConnectionID()
	conn, ok := s.connections[connID]
	if !ok {
		log.Printf("Failed to acknowledge disconnect for connection %d. Connection does not exist", connID)

	} else {
		conn.TCPConn.Close()
		delete(s.connections, connID)
	}
	empty := pb.Empty{}
	return &empty, nil
}

// This function will be responsible for continuously receiving tunnel data
// from the client and sending it to our connection
func (s *server) CreateByteStream(stream pb.GTunnel_CreateByteStreamServer) error {
	s.tStream = stream
	for {
		bMessage := new(pb.BytesMessage)
		err := stream.RecvMsg(bMessage)
		if err != nil {
			log.Printf("Failed to receive a message from the tunnel stream")
			break
		}
		connection, ok := s.connections[bMessage.ConnectionID]
		if !ok {
			log.Printf("Recieved data for a non existent connection")
		}
		bytes := bMessage.GetContent()
		_, err = connection.TCPConn.Write(bytes)
		if err != nil {
			log.Printf("Failed to send data to tcp connection")
		}
	}
	return nil
}

func (s *server) SendMessageToServer(ctx context.Context, message *pb.ConnectionMessage) (*pb.Empty, error) {
	connection, ok := s.connections[message.ConnectionID]
	if !ok {
		log.Printf("SendMessageToServer received ID of %d but doesn't exist.", message.ConnectionID)
	} else {
		if message.ErrStatus == 0 {
			connection.Connected <- true
		} else {
			connection.Connected <- false
		}
	}
	empty := pb.Empty{}
	return &empty, nil
}

// This function will continously read bytes from
// a tcp socket and send them over the tunnel to the server
func (s *server) sendDataToServer(connID int32) {
	connection, ok := s.connections[connID]
	if !ok {
		log.Printf("Failed to look up connection for connID: %d", connID)
	}
	for {
		bytes := make([]byte, 4096)
		bMessage := new(pb.BytesMessage)
		bRead, err := connection.TCPConn.Read(bytes)
		if err != nil {
			if err == io.EOF {
				bytes = make([]byte, 0)
			} else {
				log.Printf("Error reading from tcp connnection : %v", err)
				connection.Kill <- true
				break
			}
		}
		bMessage.ConnectionID = connID
		bMessage.Content = bytes[:bRead]
		err = s.tStream.Send(bMessage)

		if err != nil {
			log.Printf("Failed to send data over tunnel stream")
		}
		if len(bytes) == 0 {
			break
		}
	}
}

// This function will create a listening thread which will
// accept new connections and assign them a connection ID
func (s *server) runListener(lPort int, targetIP net.IP, targetPort int) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", lPort))
	if err != nil {
		log.Printf("Failed to listen on port %d : %v", lPort, err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Failed to accept on port %d: %v", lPort, err)
		}
		gConn := common.NewConnection(conn)

		message := pb.ConnectionMessage{}
		message.Port = uint32(targetPort)
		message.IpAddress = binary.BigEndian.Uint32(targetIP.To4())
		message.Operation = 1
		message.ConnectionID = s.messageCount
		s.messageCount++

		s.connections[message.ConnectionID] = *gConn
		s.output <- message
		if <-gConn.Connected {
			go s.sendDataToServer(message.ConnectionID)
		} else {
			conn.Close()
			delete(s.connections, message.ConnectionID)
		}
	}
}

// The console based UI menu to add a tunnel
func (s *server) UIAddTunnel(c *ishell.Context) {
	c.Print("Provide port to listen on: ")
	lPort, err := strconv.Atoi(c.ReadLine())

	for err != nil {
		c.Print("Provide a valid number for listening port : ")
		lPort, err = strconv.Atoi(c.ReadLine())
	}

	c.Print("Provide IP Address to route to : ")
	targetIP := net.ParseIP(c.ReadLine())
	for targetIP == nil {
		c.Print("Please provide valid IP address: ")
		targetIP = net.ParseIP(c.ReadLine())
	}

	c.Print("Provide Port to route to : ")
	targetPort, err := strconv.Atoi(c.ReadLine())
	for err != nil {
		c.Print("Provide a valid port number : ")
		targetPort, err = strconv.Atoi(c.ReadLine())
	}

	go s.runListener(lPort, targetIP, targetPort)

}

// The console based UI menu to list out all connections
func (s *server) UIListConnections(c *ishell.Context) {
	for k, _ := range s.connections {
		c.Printf("Connection ID: %d\n", k)
	}
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	s := new(server)
	s.connections = make(map[int32]common.Connection)
	s.output = make(chan pb.ConnectionMessage)

	grpcServer := grpc.NewServer(opts...)

	pb.RegisterGTunnelServer(grpcServer, s)
	go grpcServer.Serve(lis)

	shell := ishell.New()
	shell.Println("Tunnel manager")

	shell.AddCmd(&ishell.Cmd{
		Name: "addtunnel",
		Help: "Creates a tunnel",
		Func: s.UIAddTunnel,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "listconns",
		Help: "List all active tcp connections",
		Func: s.UIListConnections,
	})

	shell.Run()
}
