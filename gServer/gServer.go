package main

import (
	"context"
	"flag"
	"fmt"
	"gTunnel/common"
	pb "gTunnel/gTunnel"
	"log"
	"net"
	"strconv"

	"github.com/abiosoft/ishell"
	"google.golang.org/grpc"
)

var (
	tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "", "The TLS cert file")
	keyFile    = flag.String("key_file", "", "The TLS key file")
	jsonDBFile = flag.String("json_db_file", "", "A json file containing a list of features")
	port       = flag.Int("port", 5555, "The server port")
)

type gServer struct {
	pb.UnimplementedGTunnelServer
	endpoints       map[string]*common.Endpoint
	endpointInputs  map[string]chan *pb.EndpointControlMessage
	keyboardInput   chan pb.EndpointControlMessage
	currentEndpoint string
}

type ServerConnectionHandler struct {
	server     *gServer
	endpointId string
	tunnelId   string
}

func (s *ServerConnectionHandler) GetByteStream(connId int32) common.ByteStream {
	endpoint := s.server.endpoints[s.endpointId]
	tunnel := endpoint.GetTunnel(s.tunnelId)
	stream := tunnel.GetControlStream()
	conn := tunnel.GetConnection(connId)

	message := new(pb.TunnelControlMessage)
	message.Operation = common.TunnelCtrlConnect
	message.EndpointID = s.endpointId
	message.TunnelID = s.tunnelId
	message.ConnectionID = connId
	stream.Send(message)
	<-conn.Connected
	return conn.GetStream()
}

func (s *ServerConnectionHandler) CloseStream(connId int32) {
	endpoint := s.server.endpoints[s.endpointId]
	tunnel := endpoint.GetTunnel(s.tunnelId)
	conn := tunnel.GetConnection(connId)

	conn.Kill <- true

}

// RPC function that will listen on the output channel and send
// any messages over to the gClient.
func (s *gServer) CreateEndpointControlStream(ctrlMessage *pb.EndpointControlMessage, stream pb.GTunnel_CreateEndpointControlStreamServer) error {
	log.Printf("Endpoint connected: id: %s", ctrlMessage.EndpointID)

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// This is for accepting keyboard input, each client needs their own channel
	inputChannel := make(chan *pb.EndpointControlMessage)
	s.endpointInputs[ctrlMessage.EndpointID] = inputChannel

	s.endpoints[ctrlMessage.EndpointID] = common.NewEndpoint(ctrlMessage.EndpointID)

	for {
		select {

		case controlMessage := <-inputChannel:
			controlMessage.EndpointID = ctrlMessage.EndpointID
			stream.Send(controlMessage)
		case <-ctx.Done():
			log.Fatalf("Read timed out")
			return nil
		}
	}
}

func (s *gServer) CreateTunnelControlStream(stream pb.GTunnel_CreateTunnelControlStreamServer) error {

	tunMessage, err := stream.Recv()
	if err != nil {
		log.Printf("Failed to receive initial tun stream message: %v", err)
	}
	log.Printf("CreateTunnelControlStream: Endpoint ID: %s\tTunnelID: %s\n", tunMessage.EndpointID, tunMessage.TunnelID)

	endpoint := s.endpoints[tunMessage.EndpointID]
	tun := endpoint.GetTunnel(tunMessage.TunnelID)

	tun.SetControlStream(stream)
	tun.Start()
	<-tun.Kill
	return nil
}

func (s *gServer) CreateConnectionStream(stream pb.GTunnel_CreateConnectionStreamServer) error {
	bytesMessage, _ := stream.Recv()
	endpoint := s.endpoints[bytesMessage.EndpointID]
	tunnel := endpoint.GetTunnel(bytesMessage.TunnelID)
	conn := tunnel.GetConnection(bytesMessage.ConnectionID)
	conn.SetStream(stream)
	log.Printf("Setting byte stream for connection: %d", bytesMessage.ConnectionID)
	conn.Connected <- true
	<-conn.Kill
	log.Printf("CreateConnection stream done")
	tunnel.RemoveConnection(conn.Id)
	return nil
}

/*
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

func (s *server) SendMessageToServer(ctx context.Context, message *pb.ControlMessage) (*pb.Empty, error) {
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
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", lPort))
	if err != nil {
		log.Printf("Failed to listen on port %d : %v", lPort, err)
		return
	}

	newConns := make(chan net.Conn)

	go func(l net.Listener) {
		for {
			c, err := l.Accept()
			if err != nil {
				newConns <- nil
			}
			newConns <- c
		}
	}(ln)

	for {
		select {
		case conn := <-newConns:
			gConn := common.NewConnection(conn)

			message := pb.ControlMessage{}
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
		case <-tunnelKill:
			log.Printf("Tunnel closed")
			return nil
		}
	}
}*/

// The console based UI menu to add a tunnel
func (s *gServer) UIAddTunnel(c *ishell.Context) {
	if s.currentEndpoint == "" {
		log.Printf("No endpoint selected.")
		return
	}

	listenPort, _ := strconv.Atoi(c.Args[0])
	targetIP := net.ParseIP(c.Args[1])
	targetPort, _ := strconv.Atoi(c.Args[2])

	endpointInput, ok := s.endpointInputs[s.currentEndpoint]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. Addtunnel failed")
		return
	}

	endpoint, ok := s.endpoints[s.currentEndpoint]
	if !ok {
		log.Printf("Unable to locate endpoint. Addtunnel failed")
		return
	}

	tId := common.GenerateString(common.TunnelIDSize)

	newTunnel := common.NewTunnel(tId, 0, 0, common.IpToInt32(targetIP), uint32(targetPort))
	f := new(ServerConnectionHandler)
	f.server = s
	f.endpointId = s.currentEndpoint
	f.tunnelId = tId

	newTunnel.ConnectionHandler = f

	if !newTunnel.AddListener(int32(listenPort), s.currentEndpoint) {
		log.Printf("Failed to start listener. Returning")
		return
	}

	endpoint.AddTunnel(tId, newTunnel)

	controlMessage := new(pb.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlAddTunnel

	controlMessage.TunnelID = tId
	controlMessage.RemoteIP = common.IpToInt32(targetIP)
	controlMessage.RemotePort = uint32(targetPort)
	controlMessage.LocalIp = 0
	controlMessage.LocalPort = 0

	endpointInput <- controlMessage

}

/*
// The console based UI menu to list out all connections
func (s *server) UIListConnections(c *ishell.Context) {
	for k, _ := range s.connections {
		c.Printf("Connection ID: %d\n", k)
	}
}*/

func (s *gServer) UISetCurrentEndpoint(c *ishell.Context) {
	endpointId := c.Args[0]
	if _, ok := s.endpoints[endpointId]; ok {
		s.currentEndpoint = endpointId
		c.SetPrompt(fmt.Sprintf("(%s) >>> ", endpointId))
	} else {
		c.Printf("Endpoint %s does not exist.", endpointId)
	}
}

func (s *gServer) UIBack(c *ishell.Context) {
	if s.currentEndpoint != "" {
		s.currentEndpoint = ""
		c.SetPrompt(">>> ")
	} else {
		c.Printf("No client currently set")
	}
}

func (s *gServer) UIListTunnels(c *ishell.Context) {
	if s.currentEndpoint == "" {
		c.Printf("No endpoint selected")
		return
	}
	endpoint := s.endpoints[s.currentEndpoint]
	for key := range endpoint.GetTunnels() {
		c.Printf("Tunnel ID: %s\n", key)
	}

}

func (s *gServer) UIDeleteTunnel(c *ishell.Context) {
	if s.currentEndpoint == "" {
		c.Printf("No endpoint selected")
		return
	}

	endpoint, _ := s.endpoints[s.currentEndpoint]
	tunnels := endpoint.GetTunnels()
	var tunID string

	if len(c.Args) < 1 {
		ids := make([]string, 0, len(tunnels))

		for id := range tunnels {
			ids = append(ids, id)
		}
		choice := c.MultiChoice(ids, "Select tunnel ID")

		tunID = ids[choice]

	} else {
		tunID = c.Args[0]
	}

	c.Printf("Deleting tunnel : %s", tunID)

	endpoint.RemoveTunnel(tunID)
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	s := new(gServer)
	//s.connections = make(map[int32]common.Connection)
	s.keyboardInput = make(chan pb.EndpointControlMessage)
	s.endpointInputs = make(map[string]chan *pb.EndpointControlMessage)
	s.endpoints = make(map[string]*common.Endpoint)

	grpcServer := grpc.NewServer(opts...)

	pb.RegisterGTunnelServer(grpcServer, s)
	go grpcServer.Serve(lis)

	shell := ishell.New()
	shell.Println("Tunnel manager")

	shell.AddCmd(&ishell.Cmd{
		Name: "use",
		Help: "Select endpoint to use",
		Func: s.UISetCurrentEndpoint,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "back",
		Help: "Deselect the current endpoint",
		Func: s.UIBack,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "addtunnel",
		Help: "Creates a tunnel",
		Func: s.UIAddTunnel,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "deltunnel",
		Help: "Remove tunnel",
		Func: s.UIDeleteTunnel,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "listtunnels",
		Help: "Lists all tunnels for an endpoint",
		Func: s.UIListTunnels,
	})

	/*shell.AddCmd(&ishell.Cmd{
		Name: "listconns",
		Help: "List all active tcp connections",
		Func: s.UIListConnections,
	})*/

	shell.Run()
}
