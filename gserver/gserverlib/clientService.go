package gserverlib

import (
	"context"
	"fmt"
	"log"
	"net"

	cs "github.com/hotnops/gTunnel/grpc/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/hotnops/gTunnel/common"
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
func (s *ClientServiceServer) GetConfigurationMessage(ctx context.Context, empty *cs.GetConfigurationMessageRequest) (
	*cs.GetConfigurationMessageResponse, error) {

	token, err := common.GetBearerTokenFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	clientConfig, err := s.gServer.authStore.GetClientConfig(token)

	configMsg := new(cs.GetConfigurationMessageResponse)
	configMsg.EndpointId = clientConfig.ID
	configMsg.KillDate = 0

	return configMsg, nil

}

// CreateEndpointControl stream is a gRPC function that the client
// calls to establish a one way stream that the server uses to issue
// control messages to the remote endpoint.
func (s *ClientServiceServer) CreateEndpointControlStream(
	ctrlMessage *cs.EndpointControlMessage,
	stream cs.ClientService_CreateEndpointControlStreamServer) error {
	log.Printf("Endpoint connected: id: %s", ctrlMessage.EndpointId)

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// This is for accepting keyboard input, each client needs their own
	// channel
	inputChannel := make(chan *cs.EndpointControlMessage)
	s.gServer.endpointInputs[ctrlMessage.EndpointId] = inputChannel

	s.gServer.endpoints[ctrlMessage.EndpointId] = common.NewEndpoint()
	s.gServer.endpoints[ctrlMessage.EndpointId].SetID(ctrlMessage.EndpointId)

	for {
		select {

		case controlMessage, ok := <-inputChannel:
			if !ok {
				log.Printf(
					"Failed to read from EndpointCtrlStream channel. Exiting")
				break
			}
			controlMessage.EndpointId = ctrlMessage.EndpointId
			stream.Send(controlMessage)
		case <-ctx.Done():
			log.Printf("Endpoint disconnected: %s", ctrlMessage.EndpointId)
			endpoint, ok := s.gServer.endpoints[ctrlMessage.EndpointId]
			if !ok {
				log.Printf("Endpoint already removed: %s",
					ctrlMessage.EndpointId)
			}
			endpoint.Stop()
			delete(s.gServer.endpoints, ctrlMessage.EndpointId)
			return nil
		}
	}
}

//CreateTunnelControlStream is a gRPC function that the client will call to
// establish a bi-directional stream to relay control messages about new
// and disconnected TCP connections.
func (s *ClientServiceServer) CreateTunnelControlStream(
	stream cs.ClientService_CreateTunnelControlStreamServer) error {

	tunMessage, err := stream.Recv()
	if err != nil {
		log.Printf("Failed to receive initial tun stream message: %v", err)
	}

	endpoint := s.gServer.endpoints[tunMessage.EndpointId]
	tun, _ := endpoint.GetTunnel(tunMessage.TunnelId)

	tun.SetControlStream(stream)
	tun.Start()
	<-tun.Kill
	return nil
}

// CreateConnectionStream is a gRPC function that the client twill call to
// create a bi-directional data stream to carry data that gets delivered
// over the TCP connection.
func (s *ClientServiceServer) CreateConnectionStream(
	stream cs.ClientService_CreateConnectionStreamServer) error {
	bytesMessage, _ := stream.Recv()
	endpoint := s.gServer.endpoints[bytesMessage.EndpointId]
	tunnel, _ := endpoint.GetTunnel(bytesMessage.TunnelId)
	conn := tunnel.GetConnection(bytesMessage.ConnectionId)
	conn.SetStream(stream)
	close(conn.Connected)
	<-conn.Kill
	tunnel.RemoveConnection(conn.Id)
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
		grpc.UnaryInterceptor(common.UnaryAuthInterceptor),
		grpc.StreamInterceptor(common.StreamAuthInterceptor),
	)

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if tls == true {
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
