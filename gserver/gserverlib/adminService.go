package gserverlib

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/hotnops/gTunnel/common"
	as "github.com/hotnops/gTunnel/grpc/admin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type AdminServiceServer struct {
	as.UnimplementedAdminServiceServer
	gServer *GServer
}

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

func NewAdminServiceServer(gServer *GServer) *AdminServiceServer {
	adminServer := new(AdminServiceServer)
	adminServer.gServer = gServer
	return adminServer
}

func (s *AdminServiceServer) ClientCreate(ctx context.Context, req *as.ClientCreateRequest) (
	*as.ClientCreateResponse, error) {
	log.Printf("[*] ClientCreate called")

	ip := common.Int32ToIP(req.IpAddress)

	file, err := s.gServer.GenerateClient(
		req.Platform,
		ip.To4().String(),
		uint16(req.Port),
		req.ClientID)

	if err != nil {
		return nil, err
	}

	resp := new(as.ClientCreateResponse)

	_, err = file.Read(resp.ClientBinary)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read file")
	}

	return resp, nil
}

func (s *AdminServiceServer) ClientList(req *as.ClientListRequest,
	stream as.AdminService_ClientListServer) error {
	log.Printf("[*] ClientList called")

	endpoints := s.gServer.GetEndpoints()

	if len(endpoints) == 0 {
		return status.Error(codes.OutOfRange, "no clients exist")
	}

	for _, endpoint := range endpoints {
		resp := new(as.Client)
		resp.ClientID = endpoint.Id
		resp.Status = 1
		resp.IpAddress = 20
		resp.Port = 20
		stream.Send(resp)
	}

	return nil
}

func (s *AdminServiceServer) ClientDisconnect(ctx context.Context, req *as.ClientDisconnectRequest) (
	*as.ClientDisconnectResponse, error) {
	log.Printf("[*] ClientDisconnect called")

	id := req.ClientID

	s.gServer.DisconnectEndpoint(id)

	resp := new(as.ClientDisconnectResponse)

	return resp, nil
}

func (s *AdminServiceServer) TunnelAdd(ctx context.Context, req *as.TunnelAddRequest) (
	*as.TunnelAddResponse, error) {
	log.Printf("[*] TunnelAdd called")

	err := s.gServer.AddTunnel(
		req.ClientID,
		req.Tunnel.ID,
		req.Tunnel.Direction,
		common.Int32ToIP(req.Tunnel.ListenIP),
		req.Tunnel.ListenPort,
		common.Int32ToIP(req.Tunnel.DestinationIP),
		req.Tunnel.DestinationPort)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.TunnelAddResponse), nil
}

func (s *AdminServiceServer) TunnelList(req *as.TunnelListRequest,
	stream as.AdminService_TunnelListServer) error {
	log.Printf("[*] TunnelList called")

	clientID := req.ClientID

	endpoint, ok := s.gServer.GetEndpoint(clientID)
	if !ok {
		return status.Error(codes.InvalidArgument,
			fmt.Sprintf("clientID %s does not exist", clientID))
	}

	tunnels := endpoint.GetTunnels()

	if len(tunnels) == 0 {
		return status.Error(codes.OutOfRange,
			fmt.Sprintf("%s does not have any tunnels", clientID))
	}

	for id, tunnel := range tunnels {
		newTun := new(as.Tunnel)
		newTun.ID = id
		newTun.Direction = tunnel.GetDirection()
		newTun.ListenIP = common.IpToInt32(tunnel.GetListenIP())
		newTun.ListenPort = tunnel.GetListenPort()
		newTun.DestinationIP = common.IpToInt32(tunnel.GetDestinationIP())
		newTun.DestinationPort = tunnel.GetDestinationPort()

		stream.Send(newTun)
	}

	return nil
}

func (s *AdminServiceServer) TunnelDelete(ctx context.Context, req *as.TunnelDeleteRequest) (
	*as.TunnelDeleteResponse, error) {
	log.Printf("[*] TunnelDelete called")

	err := s.gServer.DeleteTunnel(req.ClientID, req.TunnelID)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.TunnelDeleteResponse), nil
}

func (s *AdminServiceServer) ConnectionList(req *as.ConnectionListRequest,
	stream as.AdminService_ConnectionListServer) error {
	log.Printf("[*] ConnectionList called")

	clientID := req.ClientID
	tunnelID := req.TunnelID

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
		newCon.SourceIP = common.IpToInt32(sourceIP)
		newCon.SourcePort = uint32(connection.TCPConn.LocalAddr().(*net.TCPAddr).Port)
		newCon.DestinationIP = common.IpToInt32(destIP)
		newCon.DestinationPort = uint32(connection.TCPConn.RemoteAddr().(*net.TCPAddr).Port)
		stream.Send(newCon)
	}
	return nil
}

func (s *AdminServiceServer) SocksStart(ctx context.Context, req *as.SocksStartRequest) (
	*as.SocksStartResponse, error) {
	log.Printf("[*] SocksStart called")

	clientID := req.ClientID
	socksPort := req.SocksPort

	err := s.gServer.StartProxy(clientID, socksPort)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.SocksStartResponse), nil
}

func (s *AdminServiceServer) SocksStop(ctx context.Context, req *as.SocksStopRequest) (
	*as.SocksStopResponse, error) {
	log.Printf("[*] SocksStart called")

	clientID := req.ClientID

	err := s.gServer.StopProxy(clientID)

	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return new(as.SocksStopResponse), nil
}
