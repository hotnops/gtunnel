package gserverlib

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/hotnops/gTunnel/common"
	cs "github.com/hotnops/gTunnel/grpc/client"
	"google.golang.org/grpc"
)

type contextKey string

func (c contextKey) String() string {
	return string(c)
}

type ConfiguredClient struct {
	Arch     string
	BinType  string
	Name     string
	Platform string
	Port     uint32
	Proxy    string
	Server   string
	Token    string
}

type ConnectedClient struct {
	configuredClient *ConfiguredClient
	hostname         string
	remoteAddr       string
	uniqueID         string
	connectDate      time.Time
	endpoint         *common.Endpoint
	endpointInput    chan *cs.EndpointControlMessage
}

type GServer struct {
	//endpoints        map[string]*common.Endpoint
	//endpointInputs   map[string]chan *cs.EndpointControlMessage
	configStore      *ConfigStore
	clientServer     *ClientServiceServer
	adminServer      *AdminServiceServer
	connectedClients map[string]*ConnectedClient
}

// ServerConnectionHandler TODO
type ServerConnectionHandler struct {
	server     *GServer
	endpointID string
	tunnelID   string
}

// NewGServer is a constructor that will initialize
// all gServer internal data structures and load any
// existing configuration files.
func NewGServer() *GServer {

	newServer := new(GServer)

	// Create and initialize all of the existing configured clients
	newServer.configStore = NewConfigStore()
	newServer.configStore.Initialize()

	newServer.clientServer = NewClientServiceServer(newServer)
	newServer.adminServer = NewAdminServiceServer(newServer)
	newServer.connectedClients = make(map[string]*ConnectedClient)

	return newServer
}

func NewConfiguredClient(clientData map[string]interface{}) *ConfiguredClient {
	c := new(ConfiguredClient)
	c.Arch = clientData["Arch"].(string)
	c.Name = clientData["Name"].(string)
	c.Port = clientData["Port"].(uint32)
	c.Server = clientData["Server"].(string)
	c.Token = clientData["Token"].(string)
	return c
}

// AddConnectedClient will take in a unique ID and a ConnectedClient structure
// and insert them into the connectedClients map with the unique ID as the key.
func (s *GServer) AddConnectedClient(uuid string, client *ConnectedClient) bool {
	_, ok := s.connectedClients[uuid]

	if ok {
		log.Printf("[!] Attempting to add client that already exists")
		return false
	}
	s.connectedClients[uuid] = client

	return true
}

// StreamAuthInterceptor will check for proper authorization for all
// stream based gRPC calls.
func (s *GServer) StreamAuthInterceptor(srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {

	ctx := ss.Context()

	token, uuid, err := GetClientInfoFromCtx(ctx)

	if err != nil {
		return err
	}

	client := s.configStore.GetConfiguredClient(token)

	if client == nil {
		log.Printf("[!] Invalid bearer token\n")
		return fmt.Errorf("invalid bearer token")
	}

	_, ok := s.connectedClients[uuid]

	if !ok {
		log.Printf("[!] UUID not connected\n")
		return fmt.Errorf("uuid not connected")
	}

	_ = context.WithValue(ctx, contextKey("uuid"), uuid)

	return handler(srv, ss)
}

// UnaryAuthInterceptor is called for all unary gRPC functions
// to validate that the caller is authorized.
func (s *GServer) UnaryAuthInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	token, uuid, err := GetClientInfoFromCtx(ctx)

	if err != nil {
		return nil, err
	}

	client := s.configStore.GetConfiguredClient(token)

	if client == nil {
		log.Printf("[!] Invalid bearer token\n")
		return nil, fmt.Errorf("invalid bearer token")
	}

	_, ok := s.connectedClients[uuid]

	if ok {
		log.Printf("[!] gClient with uuid: %s already connected\n", uuid)
	}

	ctx = context.WithValue(ctx, contextKey("uuid"), uuid)

	return handler(ctx, req)
}

// AddTunnel adds a tunnel to the gRPC server and then messages the gclient
// to perform actions on the other end.
func (s *GServer) AddTunnel(
	clientID string,
	tunnelID string,
	direction uint32,
	listenIP net.IP,
	listenPort uint32,
	destinationIP net.IP,
	destinationPort uint32) error {

	client, ok := s.connectedClients[clientID]

	if !ok {
		log.Printf("[!] client with uuuid: %s does not exist\n", clientID)
		return fmt.Errorf("addtunnel failed - client does not exist")
	}

	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlAddTunnel
	controlMessage.TunnelId = tunnelID
	newTunnel := common.NewTunnel(tunnelID,
		direction,
		listenIP,
		uint32(listenPort),
		destinationIP,
		uint32(destinationPort))

	if direction == common.TunnelDirectionForward {

		controlMessage.DestinationIp = common.IpToInt32(destinationIP)
		controlMessage.DestinationPort = uint32(destinationPort)
		// The client doesn't need to know what port and IP we are
		// listening on
		controlMessage.ListenIp = 0
		controlMessage.ListenPort = 0
	} else if direction == common.TunnelDirectionReverse {
		// In the case of a reverse tunnel, the client
		// doesn't need to know to where we are forwarding
		// the connection
		controlMessage.DestinationIp = 0
		controlMessage.DestinationPort = 0
		controlMessage.ListenIp = common.IpToInt32(listenIP)
		controlMessage.ListenPort = uint32(listenPort)
	} else {
		return fmt.Errorf("invalid tunnel direction")
	}

	if _, ok := client.endpoint.GetTunnel(tunnelID); ok {
		log.Printf("Tunnel ID already exists for this endpoint. Generating ID instead")
		tunnelID = common.GenerateString(common.TunnelIDSize)
	}

	f := new(ServerConnectionHandler)
	f.server = s
	f.endpointID = clientID
	f.tunnelID = tunnelID

	newTunnel.ConnectionHandler = f

	if direction == common.TunnelDirectionForward {

		if !newTunnel.AddListener(clientID) {
			log.Printf("Failed to start listener. Returning")
			return fmt.Errorf("failed to listen on port: %d", listenPort)
		}
	}

	client.endpoint.AddTunnel(tunnelID, newTunnel)

	client.endpointInput <- controlMessage

	return nil
}

// DeleteTunnel will kill all TCP connections under the tunnel
// and remove them from the list of managed tunnels.
func (s *GServer) DeleteTunnel(
	clientID string,
	tunnelID string) error {

	client, ok := s.connectedClients[clientID]

	if !ok {
		log.Printf("[!] client with uuuid: %s does not exist\n", clientID)
		return fmt.Errorf("deletetunnel failed - client does not exist")
	}

	if !client.endpoint.StopAndDeleteTunnel(tunnelID) {
		return fmt.Errorf("failed to delete tunnel")
	}

	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlDeleteTunnel
	controlMessage.TunnelId = tunnelID

	client.endpointInput <- controlMessage

	return nil
}

// DisconnectEndpoint will send a control message to the
// current endpoint to disconnect and end execution.
func (s *GServer) DisconnectEndpoint(
	clientID string) error {

	log.Printf("Disconnecting %s", clientID)

	client, ok := s.connectedClients[clientID]

	if !ok {
		log.Printf("[!] client with uuuid: %s does not exist\n", clientID)
		return fmt.Errorf("disconnectendpoint failed - client does not exist")
	}

	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlDisconnect

	client.endpointInput <- controlMessage
	return nil
}

// RegisterClient is responsible for building
// a client executable with the provided parameters.
func (s *GServer) RegisterClient(req *ConfiguredClient) error {

	err := s.configStore.AddConfiguredClient(req)

	if err != nil {
		log.Printf("[!] Failed to generate client: %s", err)
		s.configStore.DeleteConfiguredClient(req.Token)
		return err
	}
	return nil
}

// GetClientServer gets the grpc client server
func (s *GServer) GetClientServer() *ClientServiceServer {
	return s.clientServer
}

// GetEndpoint will retreive an endpoint struct with the provided endpoint ID.
func (s *GServer) GetEndpoint(clientID string) (*common.Endpoint, bool) {
	client, ok := s.connectedClients[clientID]

	if !ok {
		log.Printf("[!] client with uuuid: %s does not exist\n", clientID)
		return nil, ok
	}

	return client.endpoint, ok
}

// Start will start the client and admin gprc servers.
func (s *GServer) Start(
	clientPort int,
	adminPort int,
	tls bool,
	certFile string,
	keyFile string) {

	go s.clientServer.Start(clientPort, tls, certFile, keyFile)
	s.adminServer.Start(adminPort)
}

// StartProxy starts a proxy on the provided endpoint ID
func (s *GServer) StartProxy(
	clientID string,
	socksPort uint32) error {

	client, ok := s.connectedClients[clientID]

	if !ok {
		log.Printf("[!] client with uuuid: %s does not exist\n", clientID)
		return fmt.Errorf("startproxy failed - client does not exist")
	}

	log.Printf("Starting socks proxy on : %d", socksPort)
	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlSocksProxy
	controlMessage.ListenPort = uint32(socksPort)

	client.endpointInput <- controlMessage

	return nil
}

// StopProxy stops a proxy on the provided endpointID
func (s *GServer) StopProxy(
	clientID string) error {

	client, ok := s.connectedClients[clientID]

	if !ok {
		log.Printf("[!] client with uuuid: %s does not exist\n", clientID)
		return fmt.Errorf("stopproxy failed - client does not exist")
	}

	controlMessage := new(cs.EndpointControlMessage)
	controlMessage.Operation = common.EndpointCtrlSocksKill

	client.endpointInput <- controlMessage

	return nil
}

// Acknowledge is called  when the remote client acknowledges that a tcp connection can
// be established on the remote side.
func (s *ServerConnectionHandler) Acknowledge(tunnel *common.Tunnel,
	ctrlMessage *cs.TunnelControlMessage) common.ByteStream {

	conn := tunnel.GetConnection(ctrlMessage.ConnectionId)

	<-conn.Connected
	return conn.GetStream()
}

//CloseStream will kill a TCP connection locally
func (s *ServerConnectionHandler) CloseStream(tunnel *common.Tunnel, connID string) {

	conn := tunnel.GetConnection(connID)

	close(conn.Kill)

}

// GetByteStream will return the gRPC stream associated with a particular TCP connection.
func (s *ServerConnectionHandler) GetByteStream(tunnel *common.Tunnel,
	ctrlMessage *cs.TunnelControlMessage) common.ByteStream {

	stream := tunnel.GetControlStream()
	conn := tunnel.GetConnection(ctrlMessage.ConnectionId)

	message := new(cs.TunnelControlMessage)
	message.Operation = common.TunnelCtrlAck
	message.TunnelId = s.tunnelID
	message.ConnectionId = ctrlMessage.ConnectionId
	// Since gRPC is always client to server, we need
	// to get the client to make the byte stream connection.
	stream.Send(message)
	<-conn.Connected
	return conn.GetStream()
}
