package main

import "C"

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"

	cs "github.com/hotnops/gTunnel/grpc/client"
	"github.com/segmentio/ksuid"

	"github.com/hotnops/gTunnel/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var clientToken = "UNCONFIGURED"
var httpProxyServer = ""
var httpsProxyServer = ""
var serverAddress = "UNCONFIGURED"
var serverPort = "" // This needs to be a string to be used with -X

// ClientStreamHandler manages the context and grpc client for
// a given TCP stream.
type ClientStreamHandler struct {
	client     cs.ClientServiceClient
	gCtx       context.Context
	ctrlStream common.TunnelControlStream
}

// gClient is a structure that represents a unique gClient
type gClient struct {
	endpoint    *common.Endpoint
	ctrlStream  cs.ClientService_CreateEndpointControlStreamClient
	grpcClient  cs.ClientServiceClient
	killClient  chan bool
	gCtx        context.Context
	socksServer *common.SocksServer
}

// Acknowledge is called to indicate that the TCP connection has been
// established on the remote side of the tunnel.
func (c *ClientStreamHandler) Acknowledge(tunnel *common.Tunnel,
	ctrlMessage *cs.TunnelControlMessage) common.ByteStream {
	return c.GetByteStream(tunnel, ctrlMessage)
}

// CloseStream does nothing.
func (c *ClientStreamHandler) CloseStream(tunnel *common.Tunnel, connID string) {
	return
}

// GetByteStream is responsible for returning a bi-directional gRPC
// stream that will be used for relaying TCP data.
func (c *ClientStreamHandler) GetByteStream(tunnel *common.Tunnel,
	ctrlMessage *cs.TunnelControlMessage) common.ByteStream {

	stream, err := c.client.CreateConnectionStream(c.gCtx)
	if err != nil {
		return nil
	}

	// Once byte stream is open, send an initial message
	// with all the appropriate IDs
	bytesMessage := new(cs.BytesMessage)
	bytesMessage.TunnelId = ctrlMessage.TunnelId
	bytesMessage.ConnectionId = ctrlMessage.ConnectionId

	stream.Send(bytesMessage)

	// Lastly, forward the control message to the
	// server to indicate we have acknowledged the connection
	ctrlMessage.Operation = common.TunnelCtrlAck
	c.ctrlStream.Send(ctrlMessage)

	return stream
}

// receiveClientControlMessages is responsible for reading
// all control messages and dealing with them appropriately.
func (c *gClient) receiveClientControlMessages() {
	ctrlMessageChan := make(chan *cs.EndpointControlMessage)

	go func(c cs.ClientService_CreateEndpointControlStreamClient) {
		for {
			message, err := c.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				os.Exit(0)
			}
			ctrlMessageChan <- message
		}
	}(c.ctrlStream)

	for {
		select {
		case message := <-ctrlMessageChan:
			operation := message.Operation
			if operation == common.EndpointCtrlAddTunnel {
				var direction = 0
				if message.ListenPort == 0 {
					direction = common.TunnelDirectionForward
				} else {
					direction = common.TunnelDirectionReverse
				}
				newTunnel := common.NewTunnel(message.TunnelId,
					uint32(direction),
					common.Int32ToIP(message.ListenIp),
					message.ListenPort,
					common.Int32ToIP(message.DestinationIp),
					message.DestinationPort)

				f := new(ClientStreamHandler)
				f.client = c.grpcClient
				f.gCtx = c.gCtx

				if direction == common.TunnelDirectionReverse {
					newTunnel.AddListener(int32(message.ListenPort), c.endpoint.Id)
				}

				tStream, _ := c.grpcClient.CreateTunnelControlStream(c.gCtx)

				// Once we have the control stream, set it in our client handler
				f.ctrlStream = tStream
				newTunnel.ConnectionHandler = f
				newTunnel.SetControlStream(tStream)

				// Send a message through the new stream
				// to let the server know the ID specifics
				tMsg := new(cs.TunnelControlMessage)
				tMsg.TunnelId = message.TunnelId
				tStream.Send(tMsg)

				c.endpoint.AddTunnel(message.TunnelId, newTunnel)
				newTunnel.Start()

			} else if operation == common.EndpointCtrlDeleteTunnel {
				c.endpoint.StopAndDeleteTunnel(message.TunnelId)
			} else if operation == common.EndpointCtrlSocksProxy {
				message.Operation = common.EndpointCtrlSocksProxyAck
				message.ErrorStatus = 0
				if c.socksServer != nil {
					message.ErrorStatus = 1
				}

				c.socksServer = common.NewSocksServer(message.ListenPort)
				if !c.socksServer.Start() {
					message.ErrorStatus = 2
				}
				//c.ctrlStream.SendMsg(message)
			} else if operation == common.EndpointCtrlSocksKill {
				if c.socksServer != nil {
					c.socksServer.Stop()
					c.socksServer = nil
				}
			} else if operation == common.EndpointCtrlDisconnect {
				close(c.killClient)
			}

		case <-c.killClient:
			os.Exit(0)
		}
	}
}

//export ExportMain
func ExportMain() {
	main()
}

func main() {
	var err error
	var cancel context.CancelFunc

	uniqueID := ksuid.New().String()

	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	if len(httpProxyServer) > 0 {
		os.Setenv("HTTP_PROXY", httpProxyServer)
	}

	if len(httpsProxyServer) > 0 {
		os.Setenv("HTTPS_PROXY", httpsProxyServer)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)),
		grpc.WithPerRPCCredentials(common.NewToken(clientToken+"-"+uniqueID)))

	gClient := new(gClient)
	gClient.endpoint = common.NewEndpoint()
	gClient.killClient = make(chan bool)
	gClient.socksServer = nil

	serverAddr := fmt.Sprintf("%s:%s", serverAddress, serverPort)

	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return
	}
	defer conn.Close()

	req := new(cs.GetConfigurationMessageRequest)

	req.Hostname, _ = os.Hostname()

	gClient.grpcClient = cs.NewClientServiceClient(conn)
	gClient.gCtx, cancel = context.WithCancel(context.Background())
	defer cancel()

	_, err = gClient.grpcClient.GetConfigurationMessage(gClient.gCtx, req)
	if err != nil {
		return
	}

	conMsg := new(cs.EndpointControlMessage)
	gClient.ctrlStream, err = gClient.grpcClient.CreateEndpointControlStream(gClient.gCtx, conMsg)

	if err != nil {
		return
	}

	go gClient.receiveClientControlMessages()
	<-gClient.killClient
}
