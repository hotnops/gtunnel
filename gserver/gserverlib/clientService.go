package gserverlib

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	cs "github.com/hotnops/gTunnel/grpc/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/hotnops/gTunnel/common"
	"google.golang.org/grpc/peer"
)

type ClientServiceServer struct {
	cs.UnimplementedClientServiceServer
	gServer *GServer
}

func NewClientServiceServer(gserver *GServer) *ClientServiceServer {
	c := new(ClientServiceServer)
	c.gServer = gserver
	return c
}

// GetConfigurationMessage returns the client ID and kill date to a
// gClient based on the bearer token provided.
func (s *ClientServiceServer) GetConfigurationMessage(ctx context.Context, req *cs.GetConfigurationMessageRequest) (
	*cs.GetConfigurationMessageResponse, error) {

	token, uuid, err := GetClientInfoFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	clientConfig := s.gServer.configStore.GetConfiguredClient(token)

	if clientConfig == nil {
		log.Printf("[!] Failed to lookup configured client with key: %s\n", token)
		return nil, fmt.Errorf("token does not exist in client configuration")
	}

	peerInfo, ok := peer.FromContext(ctx)

	if !ok {
		log.Printf("[!] Failed to get peer info.")
		return nil, fmt.Errorf("getting info from peer context failed")
	}

	log.Printf("[*] New client connected: %s\n%s\n%s\n", clientConfig.Name, uuid, peerInfo.Addr.String())

	connectedclient := new(ConnectedClient)
	connectedclient.uniqueID = uuid
	connectedclient.remoteAddr = peerInfo.Addr.String()
	connectedclient.hostname = req.Hostname
	connectedclient.configuredClient = clientConfig
	connectedclient.connectDate = time.Now()
	connectedclient.endpoint = common.NewEndpoint()
	connectedclient.endpointInput = make(chan *cs.EndpointControlMessage)

	s.gServer.AddConnectedClient(uuid, connectedclient)

	configMsg := new(cs.GetConfigurationMessageResponse)

	return configMsg, nil

}

// CreateEndpointControl stream is a gRPC function that the client
// calls to establish a one way stream that the server uses to issue
// control messages to the remote endpoint.
func (s *ClientServiceServer) CreateEndpointControlStream(
	ctrlMessage *cs.EndpointControlMessage,
	stream cs.ClientService_CreateEndpointControlStreamServer) error {

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	_, uuid, err := GetClientInfoFromCtx(ctx)

	if err != nil {
		return err
	}

	client, ok := s.gServer.connectedClients[uuid]

	if !ok {
		log.Printf("[!] UUID does not exist to create control stream")
		return fmt.Errorf("uuid does not exist")
	}

	for {
		select {

		case controlMessage, ok := <-client.endpointInput:
			if !ok {
				log.Printf(
					"Failed to read from EndpointCtrlStream channel. Exiting")
				break
			}
			stream.Send(controlMessage)
		case <-ctx.Done():
			log.Printf("Endpoint disconnected: %s", uuid)
			client, ok := s.gServer.connectedClients[uuid]
			if !ok {
				log.Printf("Endpoint already removed: %s",
					uuid)
			}
			client.endpoint.Stop()
			delete(s.gServer.connectedClients, uuid)
			return nil
		}
	}
}

//CreateTunnelControlStream is a gRPC function that the client will call to
// establish a bi-directional stream to relay control messages about new
// and disconnected TCP connections.
func (s *ClientServiceServer) CreateTunnelControlStream(
	stream cs.ClientService_CreateTunnelControlStreamServer) error {

	_, uuid, err := GetClientInfoFromCtx(stream.Context())

	if err != nil {
		return err
	}

	client, ok := s.gServer.connectedClients[uuid]

	if !ok {
		log.Printf("[!] CreateTunnelControl: uuid doesn't exist: %s\n", uuid)
		return fmt.Errorf("uuid does not exist")
	}

	tunMessage, err := stream.Recv()
	if err != nil {
		log.Printf("Failed to receive initial tun stream message: %v", err)
	}

	tun, ok := client.endpoint.GetTunnel(tunMessage.TunnelId)

	if !ok {
		log.Printf("[!] Received tunnel messsage for ID that doesn't exist")
		return fmt.Errorf("failed to establish tunnel")
	}

	tun.SetControlStream(stream)
	tun.Start()
	<-tun.Kill
	return nil
}

// CreateConnectionStream is a gRPC function that the client will call to
// create a bi-directional data stream to carry data that gets delivered
// over the TCP connection.
func (s *ClientServiceServer) CreateConnectionStream(
	stream cs.ClientService_CreateConnectionStreamServer) error {

	_, uuid, err := GetClientInfoFromCtx(stream.Context())

	if err != nil {
		return err
	}

	client, ok := s.gServer.connectedClients[uuid]

	if !ok {
		log.Printf("[!] CreateTunnelControl: uuid doesn't exist: %s\n", uuid)
		return fmt.Errorf("uuid does not exist")
	}

	bytesMessage, _ := stream.Recv()
	tunnel, ok := client.endpoint.GetTunnel(bytesMessage.TunnelId)

	if !ok {
		log.Printf("[!] Got a ByteMessage for a non-existent tunnel: %s\n",
			bytesMessage.TunnelId)
		return fmt.Errorf("invalid tunnel id")
	}

	conn := tunnel.GetConnection(bytesMessage.ConnectionId)

	conn.SetStream(stream)
	close(conn.Connected)
	<-conn.Kill
	tunnel.RemoveConnection(conn.ID)
	return nil
}

// Start starts the grpc client service.
func (s *ClientServiceServer) Start(
	port int,
	tls bool,
	certFile string,
	keyFile string) {

	log.Printf("[*] Starting client grpc server on port: %d\n", port)
	var opts []grpc.ServerOption
	opts = append(opts,
		grpc.UnaryInterceptor(s.gServer.UnaryAuthInterceptor),
		grpc.StreamInterceptor(s.gServer.StreamAuthInterceptor),
	)

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if tls {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)

		if err != nil {
			log.Fatalf("Failed to load TLS certificates.")
		}

		log.Printf("Successfully loaded key/certificate pair")
		opts = append(opts, grpc.Creds(creds))
	} else {
		log.Printf("[!] Starting gServer without TLS!")
	}

	grpcServer := grpc.NewServer(opts...)

	cs.RegisterClientServiceServer(grpcServer, s)

	grpcServer.Serve(lis)

}
