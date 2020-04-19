package main

import (
	"context"
	"flag"
	"fmt"
	"gTunnel/common"
	pb "gTunnel/gTunnel"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/abiosoft/ishell"
	"google.golang.org/grpc"
)

var (
	tls        = flag.Bool("tls", true, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "tls/cert", "The TLS cert file")
	keyFile    = flag.String("key_file", "tls/key", "The TLS key file")
	jsonDBFile = flag.String("json_db_file", "", "A json file containing a list of features")
	port       = flag.Int("port", 443, "The server port")
)

type gServer struct {
	pb.UnimplementedGTunnelServer
	endpoints       map[string]*common.Endpoint
	endpointInputs  map[string]chan *pb.EndpointControlMessage
	keyboardInput   chan pb.EndpointControlMessage
	currentEndpoint string
	shell           *ishell.Shell
}

type ServerConnectionHandler struct {
	server     *gServer
	endpointId string
	tunnelId   string
}

// GetByteStream will return the gRPC stream associated with a particular TCP connection.
func (s *ServerConnectionHandler) GetByteStream(ctrlMessage *pb.TunnelControlMessage) common.ByteStream {
	endpoint := s.server.endpoints[s.endpointId]
	tunnel, ok := endpoint.GetTunnel(s.tunnelId)
	if !ok {
		log.Printf("Failed to lookup tunnel.")
		return nil
	}
	stream := tunnel.GetControlStream()
	conn := tunnel.GetConnection(ctrlMessage.ConnectionID)

	message := new(pb.TunnelControlMessage)
	message.Operation = common.TunnelCtrlAck
	message.EndpointID = s.endpointId
	message.TunnelID = s.tunnelId
	message.ConnectionID = ctrlMessage.ConnectionID
	// Since gRPC is always client to server, we need
	// to get the client to make the byte stream connection.
	stream.Send(message)
	<-conn.Connected
	return conn.GetStream()
}

// Acknowledge is called  when the remote client acknowledges that a tcp connection can
// be established on the remote side.
func (s *ServerConnectionHandler) Acknowledge(ctrlMessage *pb.TunnelControlMessage) common.ByteStream {
	endpoint := s.server.endpoints[ctrlMessage.EndpointID]
	tunnel, _ := endpoint.GetTunnel(ctrlMessage.TunnelID)
	conn := tunnel.GetConnection(ctrlMessage.ConnectionID)

	<-conn.Connected
	return conn.GetStream()
}

//CloseStream will kill a TCP connection locally
func (s *ServerConnectionHandler) CloseStream(connId int32) {
	endpoint := s.server.endpoints[s.endpointId]
	tunnel, _ := endpoint.GetTunnel(s.tunnelId)
	conn := tunnel.GetConnection(connId)

	close(conn.Kill)

}

// CreateEndpointControl stream is a gRPC function that the client
// calls to establish a one way stream that the server uses to issue
// control messages to the remote endpoint.
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

		case controlMessage, ok := <-inputChannel:
			if !ok {
				log.Printf("Failed to read from EndpointCtrlStream channel. Exiting")
				break
			}
			controlMessage.EndpointID = ctrlMessage.EndpointID
			stream.Send(controlMessage)
		case <-ctx.Done():
			log.Printf("Endpoint disconnected: %s", ctrlMessage.EndpointID)
			endpoint, ok := s.endpoints[ctrlMessage.EndpointID]
			if !ok {
				log.Printf("Endpoint already removed: %s", ctrlMessage.EndpointID)
			}
			endpoint.Stop()
			delete(s.endpoints, ctrlMessage.EndpointID)
			if s.currentEndpoint == ctrlMessage.EndpointID {
				s.shell.SetPrompt(">>> ")
			}
			return nil
		}
	}
}

//CreateTunnelControlStream is a gRPC function that the client will call to
// establish a bi-directional stream to relay control messages about new
// and disconnected TCP connections.
func (s *gServer) CreateTunnelControlStream(stream pb.GTunnel_CreateTunnelControlStreamServer) error {

	tunMessage, err := stream.Recv()
	if err != nil {
		log.Printf("Failed to receive initial tun stream message: %v", err)
	}

	endpoint := s.endpoints[tunMessage.EndpointID]
	tun, _ := endpoint.GetTunnel(tunMessage.TunnelID)

	tun.SetControlStream(stream)
	tun.Start()
	<-tun.Kill
	return nil
}

// CreateconnectionStream is a gRPC function that the clien twill call to
// create a bi-directional data stream to carry data that gets delivered
// over the TCP connection.
func (s *gServer) CreateConnectionStream(stream pb.GTunnel_CreateConnectionStreamServer) error {
	bytesMessage, _ := stream.Recv()
	endpoint := s.endpoints[bytesMessage.EndpointID]
	tunnel, _ := endpoint.GetTunnel(bytesMessage.TunnelID)
	conn := tunnel.GetConnection(bytesMessage.ConnectionID)
	conn.SetStream(stream)
	close(conn.Connected)
	<-conn.Kill
	tunnel.RemoveConnection(conn.Id)
	return nil
}

// UIAddTunnel is the UI function for adding a tunnel
func (s *gServer) UIAddTunnel(c *ishell.Context) {
	if s.currentEndpoint == "" {
		log.Printf("No endpoint selected.")
		return
	}

	if len(c.Args) < 4 {
		log.Printf("Usage: addtunnel (local|remote) listenPort destinationIP destinationPort")
		return
	}

	direction := strings.ToLower(c.Args[0])
	listenPort, _ := strconv.Atoi(c.Args[1])
	targetIP := net.ParseIP(c.Args[2])
	targetPort, _ := strconv.Atoi(c.Args[3])
	var tID string
	var newTunnel *common.Tunnel

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

	if len(c.Args) > 4 {
		tID = c.Args[4]
		if _, ok := endpoint.GetTunnel(tID); ok {
			log.Printf("Tunnel ID already exists for this endpoint. Generating ID instead")
			tID = common.GenerateString(common.TunnelIDSize)
		}

	} else {
		tID = common.GenerateString(common.TunnelIDSize)
	}

	controlMessage := new(pb.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlAddTunnel
	controlMessage.TunnelID = tID
	newTunnel = common.NewTunnel(tID, 0, 0, common.IpToInt32(targetIP), uint32(targetPort))

	if strings.HasPrefix(direction, "l") {

		controlMessage.RemoteIP = common.IpToInt32(targetIP)
		controlMessage.RemotePort = uint32(targetPort)
		controlMessage.LocalIp = 0
		controlMessage.LocalPort = 0
	} else if strings.HasPrefix(direction, "r") {

		controlMessage.RemoteIP = 0
		controlMessage.RemotePort = 0
		controlMessage.LocalIp = 0
		controlMessage.LocalPort = uint32(listenPort)
	}

	f := new(ServerConnectionHandler)
	f.server = s
	f.endpointId = s.currentEndpoint
	f.tunnelId = tID

	newTunnel.ConnectionHandler = f

	if strings.HasPrefix(direction, "l") {

		if !newTunnel.AddListener(int32(listenPort), s.currentEndpoint) {
			log.Printf("Failed to start listener. Returning")
			return
		}
	}

	endpoint.AddTunnel(tID, newTunnel)

	endpointInput <- controlMessage

}

// UISetCurrentEndpoint will change the UI prompt and indicate
// what endpoint on which we should be operating.
func (s *gServer) UISetCurrentEndpoint(c *ishell.Context) {
	endpointId := c.Args[0]
	if _, ok := s.endpoints[endpointId]; ok {
		s.currentEndpoint = endpointId
		c.SetPrompt(fmt.Sprintf("(%s) >>> ", endpointId))
	} else {
		c.Printf("Endpoint %s does not exist.", endpointId)
	}
}

// UIBack will clear the current endpoint
func (s *gServer) UIBack(c *ishell.Context) {
	if s.currentEndpoint != "" {
		s.currentEndpoint = ""
		c.SetPrompt(">>> ")
	} else {
		c.Printf("No client currently set")
	}
}

// UIListTunnels will list all tunnels related to the
// current endpoint.
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

// UIGenerateClient is responsible for building
// a client executable with the provided parameters.
func (s *gServer) UIGenerateClient(c *ishell.Context) {

	const (
		PLATFORM = iota
		SERVERADDRESS
		SERVERPORT
		ID
		RETRYCOUNT
		RETRYPERIOD
	)

	if len(c.Args) < 3 {
		log.Printf("Usage: platform configclient serverAddress serverPort (id) (retryCount) (retryPeriod)")
		return
	}

	platform := c.Args[PLATFORM]
	serverAddress := c.Args[SERVERADDRESS]
	serverPort, err := strconv.Atoi(c.Args[SERVERPORT])
	if err != nil {
		log.Printf("Invalid port specified.")
		return
	}

	id := common.GenerateString(8)

	if len(c.Args) == ID+1 {
		id = c.Args[ID]
	}

	outputPath := fmt.Sprintf("configured/%s", id)

	if platform == "win" {
		exec.Command("set GOOS=windows")
		exec.Command("set GOARCH=386")
		outputPath = fmt.Sprintf("configured/%s.exe", id)
	}

	flagString := fmt.Sprintf("-s -w -X main.ID=%s -X main.serverAddress=%s -X main.serverPort=%d", id, serverAddress, serverPort)

	cmd := exec.Command("go", "build", "-ldflags", flagString, "-o", outputPath, "gClient/gClient.go")
	cmd.Env = os.Environ()
	if platform == "win" {
		cmd.Env = append(cmd.Env, "GOOS=windows")
		cmd.Env = append(cmd.Env, "GOARCH=386")
	}
	err = cmd.Run()
}

// UIDelete tunnel will kill all TCP connections under the tunnel
// and remove them from the list of managed tunnels.
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

// UIDisconnectEndpoint will send a control message to the
// current endpoint to disconnect and end execution.
func (s *gServer) UIDisconnectEndpoint(c *ishell.Context) {
	var ID string
	if s.currentEndpoint == "" {
		ID = c.Args[0]
	} else {
		ID = s.currentEndpoint
	}
	log.Printf("Disconnecting %s", ID)
	endpointInput, ok := s.endpointInputs[ID]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. Addtunnel failed")
		return
	}

	controlMessage := new(pb.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlDisconnect

	endpointInput <- controlMessage

}

func (s *gServer) UIStartProxy(c *ishell.Context) {
	if s.currentEndpoint == "" {
		log.Printf("No enndpoint selected.")
		return
	}

	if len(c.Args) < 1 {
		log.Printf("Usage: socks remotePort")
		return
	}

	remotePort, err := strconv.Atoi(c.Args[0])
	if err != nil {
		log.Printf("Invalid remotePort")
		return
	}

	endpointInput, ok := s.endpointInputs[s.currentEndpoint]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. socks failed")
		return
	}

	log.Printf("Starting socks proxy on : %d", remotePort)
	controlMessage := new(pb.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlSocksProxy
	controlMessage.RemotePort = uint32(remotePort)

	endpointInput <- controlMessage
}

func (s *gServer) UIStopProxy(c *ishell.Context) {
	if s.currentEndpoint == "" {
		log.Printf("No enndpoint selected.")
		return
	}

	endpointInput, ok := s.endpointInputs[s.currentEndpoint]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. socks failed")
		return
	}

	controlMessage := new(pb.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlSocksKill

	endpointInput <- controlMessage
}

// What it do
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
	shell.Println(`
       ___________ ____ ___ _______    _______   ___________.____     
   ___ \__    ___/|    |   \\      \   \      \  \_   _____/|    |    
  / ___\ |    |   |    |   //   |   \  /   |   \  |    __)_ |    |    
 / /_/  >|    |   |    |  //    |    \/    |    \ |        \|    |___ 
 \___  / |____|   |______/ \____|__  /\____|__  //_______  /|_______ \
/_____/                            \/         \/         \/         \/

`)

	shell.AddCmd(&ishell.Cmd{
		Name: "use",
		Help: "Select endpoint to use",
		Func: s.UISetCurrentEndpoint,
		Completer: func([]string) []string {
			keys := make([]string, len(s.endpoints))
			for k := range s.endpoints {
				keys = append(keys, k)
			}
			return keys
		},
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
		Completer: func([]string) []string {

			if s.currentEndpoint == "" {
				return nil
			}
			endpoint := s.endpoints[s.currentEndpoint]
			tunnels := endpoint.GetTunnels()
			keys := make([]string, len(tunnels))
			for k := range tunnels {
				keys = append(keys, k)
			}
			return keys
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "listtunnels",
		Help: "Lists all tunnels for an endpoint",
		Func: s.UIListTunnels,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "socks",
		Help: "Starts a socks proxy on the remote endpoints",
		Func: s.UIStartProxy,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "sockskill",
		Help: "Stops a socks proxy on the remote endpoints",
		Func: s.UIStopProxy,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "configclient",
		Help: "Configure a gClient",
		Func: s.UIGenerateClient,
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "disconnect",
		Help: "Disconnect a gClient from the server",
		Func: s.UIDisconnectEndpoint,
		Completer: func([]string) []string {
			if s.currentEndpoint != "" {
				return nil
			}
			keys := make([]string, len(s.endpoints))
			for k := range s.endpoints {
				keys = append(keys, k)
			}
			return keys
		},
	})

	s.shell = shell

	/*shell.AddCmd(&ishell.Cmd{
		Name: "listconns",
		Help: "List all active tcp connections",
		Func: s.UIListConnections,
	})*/

	shell.Run()
}
