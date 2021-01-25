package gserverlib

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/hotnops/gTunnel/common"
	as "github.com/hotnops/gTunnel/grpc/admin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// AdminServiceServer is a structure that implements all of the
// grpc functions for the AdminServiceServer
type AdminServiceServer struct {
	as.UnimplementedAdminServiceServer
	gServer *GServer
}

// NewAdminServiceServer is a constructor that returns an AdminServiceServer
// grpc server.
func NewAdminServiceServer(gServer *GServer) *AdminServiceServer {
	adminServer := new(AdminServiceServer)
	adminServer.gServer = gServer
	return adminServer
}

// ClientCreate will create a gClient binary and send it back in a binary stream.
func (s *AdminServiceServer) ClientCreate(req *as.ClientCreateRequest,
	stream as.AdminService_ClientCreateServer) error {
	log.Printf("[*] ClientCreate called")

	ip := common.Int32ToIP(req.IpAddress)

	filePath, err := s.gServer.GenerateClient(
		req.Platform,
		ip.To4().String(),
		uint16(req.Port),
		req.ClientId)

	if err != nil {
		return err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open generated client")
	}
	defer f.Close()

	for {
		data := make([]byte, 4096)
		bytesRead, err := f.Read(data)
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("error reading file")
		} else {
			if bytesRead != 4096 {
				data = data[0:bytesRead]
			}
			bs := new(as.ByteStream)
			bs.Data = data
			stream.Send(bs)
		}
	}

	return nil
}

// ClientDisconnect will disconnect a gClient from gServer.
func (s *AdminServiceServer) ClientDisconnect(ctx context.Context, req *as.ClientDisconnectRequest) (
	*as.ClientDisconnectResponse, error) {
	log.Printf("[*] ClientDisconnect called")

	id := req.ClientId

	s.gServer.DisconnectEndpoint(id)

	resp := new(as.ClientDisconnectResponse)

	return resp, nil
}

// ClientList will list all configured clients for the gServer and their
// connection status as well as the configured ip, port, and bearer token
func (s *AdminServiceServer) ClientList(req *as.ClientListRequest,
	stream as.AdminService_ClientListServer) error {
	log.Printf("[*] ClientList called")

	clients := s.gServer.connectedClients

	if len(clients) == 0 {
		return status.Error(codes.OutOfRange, "no clients exist")
	}

	for _, client := range clients {
		resp := new(as.Client)
		resp.Name = client.configuredClient.Name
		resp.ClientId = client.uniqueID
		resp.Status = 1
		resp.RemoteAddress = client.remoteAddr
		resp.Hostname = client.hostname
		resp.ConnectDate = client.connectDate.String()
		stream.Send(resp)
	}

	return nil
}

// ConnectionList will list all the connections associated with the provided
// tunnel ID.
func (s *AdminServiceServer) ConnectionList(req *as.ConnectionListRequest,
	stream as.AdminService_ConnectionListServer) error {
	log.Printf("[*] ConnectionList called")

	clientID := req.ClientId
	tunnelID := req.TunnelId

	endpoint, ok := s.gServer.GetEndpoint(clientID)

	if !ok {
		return status.Errorf(codes.NotFound,
			fmt.Sprintf("client %s does not exist", clientID))
	}

	tunnel, ok := endpoint.GetTunnel(tunnelID)

	if !ok {
		return status.Errorf(codes.NotFound,
			fmt.Sprintf("tunnel %s does not exist", tunnelID))
	}

	connections := tunnel.GetConnections()

	if len(connections) == 0 {
		return status.Errorf(codes.OutOfRange,
			fmt.Sprint("no connections exist for tunnel %s", tunnelID))
	}

	for _, connection := range connections {
		newCon := new(as.Connection)
		sourceIP := connection.TCPConn.LocalAddr().(*net.TCPAddr).IP
		destIP := connection.TCPConn.RemoteAddr().(*net.TCPAddr).IP
		newCon.SourceIp = common.IpToInt32(sourceIP)
		newCon.SourcePort = uint32(connection.TCPConn.LocalAddr().(*net.TCPAddr).Port)
		newCon.DestinationIp = common.IpToInt32(destIP)
		newCon.DestinationPort = uint32(connection.TCPConn.RemoteAddr().(*net.TCPAddr).Port)
		stream.Send(newCon)
	}
	return nil
}

// SocksStart will start a Socksv5 proxy server on the provided client ID
func (s *AdminServiceServer) SocksStart(ctx context.Context,
	req *as.SocksStartRequest) (
	*as.SocksStartResponse, error) {
	log.Printf("[*] SocksStart called")

	clientID := req.ClientId
	socksPort := req.SocksPort

	err := s.gServer.StartProxy(clientID, socksPort)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.SocksStartResponse), nil
}

// SocksStop will stop a SocksV5 proxy server running on the provided client ID.
func (s *AdminServiceServer) SocksStop(ctx context.Context,
	req *as.SocksStopRequest) (
	*as.SocksStopResponse, error) {
	log.Printf("[*] SocksStart called")

	clientID := req.ClientId

	err := s.gServer.StopProxy(clientID)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.SocksStopResponse), nil
}

// Start will start the grpc server
func (s *AdminServiceServer) Start(port int) {
	log.Printf("[*] Starting admin grpc server on port: %d\n", port)
	grpcServer := grpc.NewServer()

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	as.RegisterAdminServiceServer(grpcServer, s)
	reflection.Register(grpcServer)

	grpcServer.Serve(lis)
}

// TunnelAdd adds a tunnel to an endpoint specified in the request.
func (s *AdminServiceServer) TunnelAdd(ctx context.Context, req *as.TunnelAddRequest) (
	*as.TunnelAddResponse, error) {
	log.Printf("[*] TunnelAdd called")

	if req.Tunnel.Id == "" {
		req.Tunnel.Id = common.GenerateString(8)
	}

	err := s.gServer.AddTunnel(
		req.ClientId,
		req.Tunnel.Id,
		req.Tunnel.Direction,
		common.Int32ToIP(req.Tunnel.ListenIp),
		req.Tunnel.ListenPort,
		common.Int32ToIP(req.Tunnel.DestinationIp),
		req.Tunnel.DestinationPort)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.TunnelAddResponse), nil
}

// TunnelDelete deletes a tunnel with the provided tunnel ID
func (s *AdminServiceServer) TunnelDelete(ctx context.Context, req *as.TunnelDeleteRequest) (
	*as.TunnelDeleteResponse, error) {
	log.Printf("[*] TunnelDelete called")

	err := s.gServer.DeleteTunnel(req.ClientId, req.TunnelId)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.TunnelDeleteResponse), nil
}

// TunnelList lists all tunnels associated with the provided client ID.
func (s *AdminServiceServer) TunnelList(req *as.TunnelListRequest,
	stream as.AdminService_TunnelListServer) error {
	log.Printf("[*] TunnelList called")

	clientID := req.ClientId

	endpoint, ok := s.gServer.GetEndpoint(clientID)
	if !ok {
		return status.Error(codes.InvalidArgument,
			fmt.Sprintf("Client_ID %s does not exist", clientID))
	}

	tunnels := endpoint.GetTunnels()

	if len(tunnels) == 0 {
		return status.Error(codes.OutOfRange,
			fmt.Sprintf("%s does not have any tunnels", clientID))
	}

	for id, tunnel := range tunnels {
		newTun := new(as.Tunnel)
		newTun.Id = id
		newTun.Direction = tunnel.GetDirection()
		newTun.ListenIp = common.IpToInt32(tunnel.GetListenIP())
		newTun.ListenPort = tunnel.GetListenPort()
		newTun.DestinationIp = common.IpToInt32(tunnel.GetDestinationIP())
		newTun.DestinationPort = tunnel.GetDestinationPort()

		stream.Send(newTun)
	}

	return nil
}
