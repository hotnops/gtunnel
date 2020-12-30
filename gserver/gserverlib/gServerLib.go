package gserverlib

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"

	"github.com/hotnops/gTunnel/common"
	cs "github.com/hotnops/gTunnel/grpc/client"
)

type GServer struct {
	endpoints      map[string]*common.Endpoint
	endpointInputs map[string]chan *cs.EndpointControlMessage
	authStore      *common.AuthStore
	clientServer   *ClientServiceServer
	adminServer    *AdminServiceServer
}

// ServerConnectionHandler TODO
type ServerConnectionHandler struct {
	server     *GServer
	endpointID string
	tunnelID   string
}

func NewGServer() *GServer {

	newServer := new(GServer)
	newServer.endpointInputs = make(map[string]chan *cs.EndpointControlMessage)
	newServer.endpoints = make(map[string]*common.Endpoint)
	newServer.authStore, _ = common.InitializeAuthStore(common.ConfigurationFile)
	newServer.clientServer = NewClientServiceServer(newServer)
	newServer.adminServer = NewAdminServiceServer(newServer)

	return newServer
}

func (g *GServer) GetClientServer() *ClientServiceServer {
	return g.clientServer
}

func (g *GServer) GetEndpoint(endpointID string) (*common.Endpoint, bool) {
	endpoint, ok := g.endpoints[endpointID]
	return endpoint, ok
}

func (g *GServer) GetEndpoints() map[string]*common.Endpoint {
	return g.endpoints
}

func (g *GServer) Start(
	clientPort int,
	adminPort int,
	tls bool,
	certFile string,
	keyFile string) {

	go g.clientServer.Start(clientPort, tls, certFile, keyFile)
	g.adminServer.Start(adminPort)
}

// GetByteStream will return the gRPC stream associated with a particular TCP connection.
func (s *ServerConnectionHandler) GetByteStream(ctrlMessage *cs.TunnelControlMessage) common.ByteStream {
	endpoint := s.server.endpoints[s.endpointID]
	tunnel, ok := endpoint.GetTunnel(s.tunnelID)
	if !ok {
		log.Printf("Failed to lookup tunnel.")
		return nil
	}
	stream := tunnel.GetControlStream()
	conn := tunnel.GetConnection(ctrlMessage.ConnectionID)

	message := new(cs.TunnelControlMessage)
	message.Operation = common.TunnelCtrlAck
	message.EndpointID = s.endpointID
	message.TunnelID = s.tunnelID
	message.ConnectionID = ctrlMessage.ConnectionID
	// Since gRPC is always client to server, we need
	// to get the client to make the byte stream connection.
	stream.Send(message)
	<-conn.Connected
	return conn.GetStream()
}

// Acknowledge is called  when the remote client acknowledges that a tcp connection can
// be established on the remote side.
func (s *ServerConnectionHandler) Acknowledge(ctrlMessage *cs.TunnelControlMessage) common.ByteStream {
	endpoint := s.server.endpoints[ctrlMessage.EndpointID]
	tunnel, _ := endpoint.GetTunnel(ctrlMessage.TunnelID)
	conn := tunnel.GetConnection(ctrlMessage.ConnectionID)

	<-conn.Connected
	return conn.GetStream()
}

//CloseStream will kill a TCP connection locally
func (s *ServerConnectionHandler) CloseStream(connID int32) {
	endpoint := s.server.endpoints[s.endpointID]
	tunnel, _ := endpoint.GetTunnel(s.tunnelID)
	conn := tunnel.GetConnection(connID)

	close(conn.Kill)

}

func (s *GServer) AddTunnel(
	clientID string,
	tunnelID string,
	direction uint32,
	listenIP net.IP,
	listenPort uint32,
	destinationIP net.IP,
	destinationPort uint32) error {

	endpointInput, ok := s.endpointInputs[clientID]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. Addtunnel failed")
		return fmt.Errorf("addtunnel failed - endpoint doesn't exist")
	}

	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlAddTunnel
	controlMessage.TunnelID = tunnelID
	newTunnel := common.NewTunnel(tunnelID,
		direction,
		listenIP,
		uint32(listenPort),
		destinationIP,
		uint32(destinationPort))

	if direction == common.TunnelDirectionForward {

		controlMessage.DestinationIP = common.IpToInt32(destinationIP)
		controlMessage.DestinationPort = uint32(destinationPort)
		// The client doesn't need to know what port and IP we are
		// listening on
		controlMessage.ListenIP = 0
		controlMessage.ListenPort = 0
	} else if direction == common.TunnelDirectionReverse {
		// In the case of a reverse tunnel, the client
		// doesn't need to know to where we are forwarding
		// the connection
		controlMessage.DestinationIP = 0
		controlMessage.DestinationPort = 0
		controlMessage.ListenIP = common.IpToInt32(listenIP)
		controlMessage.ListenPort = uint32(listenPort)
	} else {
		return fmt.Errorf("invalid tunnel direction")
	}

	endpoint, ok := s.endpoints[clientID]
	if !ok {
		log.Printf("Unable to locate endpoint. Addtunnel failed")
		return fmt.Errorf("Endpoint doesn't exist")
	}

	if _, ok := endpoint.GetTunnel(tunnelID); ok {
		log.Printf("Tunnel ID already exists for this endpoint. Generating ID instead")
		tunnelID = common.GenerateString(common.TunnelIDSize)
	}

	f := new(ServerConnectionHandler)
	f.server = s
	f.endpointID = clientID
	f.tunnelID = tunnelID

	newTunnel.ConnectionHandler = f

	if direction == common.TunnelDirectionForward {

		if !newTunnel.AddListener(int32(listenPort), clientID) {
			log.Printf("Failed to start listener. Returning")
			return fmt.Errorf("failed to listen on port: %d", listenPort)
		}
	}

	endpoint.AddTunnel(tunnelID, newTunnel)

	endpointInput <- controlMessage

	return nil
}

// GenerateClient is responsible for building
// a client executable with the provided parameters.
func (s *GServer) GenerateClient(
	platform string,
	serverAddress string,
	serverPort uint16,
	clientID string) (*os.File, error) {

	const (
		PLATFORM = iota
		SERVERADDRESS
		SERVERPORT
		ID
		HTTPSPROXY
		HTTPPROXY
		RETRYCOUNT
		RETRYPERIOD
	)

	if clientID == "" {
		clientID = common.GenerateString(8)
	}

	token, err := s.authStore.GenerateNewClientConfig(clientID)

	outputPath := fmt.Sprintf("configured/%s", clientID)

	if platform == "win" {
		exec.Command("set GOOS=windows")
		exec.Command("set GOARCH=386")
		outputPath = fmt.Sprintf("configured/%s.exe", clientID)
	} else if platform == "mac" {
		exec.Command("set GOOS=darwin")
		exec.Command("set GOARCH=amd64")
	} else if platform == "linux" {
		exec.Command("set GOOS=linux")
	} else {
		log.Printf("[!] Invalid platform specified")
	}

	flagString := fmt.Sprintf("-s -w -X main.clientToken=%s -X main.serverAddress=%s -X main.serverPort=%d", token, serverAddress, serverPort)

	cmd := exec.Command("go", "build", "-ldflags", flagString, "-o", outputPath, "gclient/gClient.go")
	cmd.Env = os.Environ()
	if platform == "win" {
		cmd.Env = append(cmd.Env, "GOOS=windows")
		cmd.Env = append(cmd.Env, "GOARCH=386")
	}
	err = cmd.Run()
	if err != nil {
		log.Printf("[!] Failed to generate client: %s", err)
		s.authStore.DeleteClientConfig(token)
		return nil, err
	}
	return os.Open(outputPath)
}

// Delete tunnel will kill all TCP connections under the tunnel
// and remove them from the list of managed tunnels.
func (s *GServer) DeleteTunnel(
	clientID string,
	tunnelID string) error {
	endpoint, ok := s.endpoints[clientID]
	if !ok {
		return fmt.Errorf("failed to delete tunnel. endpoint does not exist")
	}
	if !endpoint.StopAndDeleteTunnel(tunnelID) {
		return fmt.Errorf("failed to delete tunnel")
	}
	return nil
}

// UIDisconnectEndpoint will send a control message to the
// current endpoint to disconnect and end execution.
func (s *GServer) DisconnectEndpoint(
	endpointID string) {

	log.Printf("Disconnecting %s", endpointID)

	endpointInput, ok := s.endpointInputs[endpointID]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. Disconnect failed.\n")
		return
	}

	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlDisconnect

	endpointInput <- controlMessage

}

func (s *GServer) StartProxy(
	endpointID string,
	socksPort uint32) error {

	endpointInput, ok := s.endpointInputs[endpointID]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. socks failed")
		return fmt.Errorf("Unable to locate endpoint input channel. socks failed")
	}

	log.Printf("Starting socks proxy on : %d", socksPort)
	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlSocksProxy
	controlMessage.ListenPort = uint32(socksPort)

	endpointInput <- controlMessage

	return nil
}

func (s *GServer) StopProxy(
	endpointID string) error {

	endpointInput, ok := s.endpointInputs[endpointID]
	if !ok {
		log.Printf("Unable to locate endpoint input channel. socks failed")
		return fmt.Errorf("Unable to locate endpoint input channel. socks failed")
	}

	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlSocksKill

	endpointInput <- controlMessage

	return nil
}
