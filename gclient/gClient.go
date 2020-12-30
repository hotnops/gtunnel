package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"

	cs "github.com/hotnops/gTunnel/grpc/client"

	"github.com/hotnops/gTunnel/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var serverAddress = "UNCONFIGURED"
var serverPort = "" // This needs to be a string to be used with -X
var clientToken = "UNCONFIGURED"

var httpProxyServer = ""
var httpsProxyServer = ""

type gClient struct {
	endpoint    *common.Endpoint
	ctrlStream  cs.ClientService_CreateEndpointControlStreamClient
	grpcClient  cs.ClientServiceClient
	killClient  chan bool
	gCtx        context.Context
	socksServer *common.SocksServer
}

type ClientStreamHandler struct {
	client     cs.ClientServiceClient
	gCtx       context.Context
	ctrlStream common.TunnelControlStream
}

// GetByteStream is responsible for returning a bi-directional gRPC
// stream that will be used for relaying TCP data.
func (c *ClientStreamHandler) GetByteStream(ctrlMessage *cs.TunnelControlMessage) common.ByteStream {
	stream, err := c.client.CreateConnectionStream(c.gCtx)
	if err != nil {
		return nil
	}

	// Once byte stream is open, send an initial message
	// with all the appropriate IDs
	bytesMessage := new(cs.BytesMessage)
	bytesMessage.EndpointID = ctrlMessage.EndpointID
	bytesMessage.TunnelID = ctrlMessage.TunnelID
	bytesMessage.ConnectionID = ctrlMessage.ConnectionID

	stream.Send(bytesMessage)

	// Lastly, forward the control message to the
	// server to indicate we have acknowledged the connection
	ctrlMessage.Operation = common.TunnelCtrlAck
	c.ctrlStream.Send(ctrlMessage)

	return stream
}

// Acknowledge is called to indicate that the TCP connection has been
// established on the remote side of the tunnel.
func (c *ClientStreamHandler) Acknowledge(ctrlMessage *cs.TunnelControlMessage) common.ByteStream {
	return c.GetByteStream(ctrlMessage)
}

// CloseStream does nothing.
func (c *ClientStreamHandler) CloseStream(connId int32) {
	return
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
				newTunnel := common.NewTunnel(message.TunnelID,
					uint32(direction),
					common.Int32ToIP(message.ListenIP),
					message.ListenPort,
					common.Int32ToIP(message.DestinationIP),
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
				tMsg.EndpointID = message.EndpointID
				tMsg.TunnelID = message.TunnelID
				tStream.Send(tMsg)

				c.endpoint.AddTunnel(message.TunnelID, newTunnel)
				newTunnel.Start()

			} else if operation == common.EndpointCtrlSocksProxy {
				message.Operation = common.EndpointCtrlSocksProxyAck
				message.ErrorStatus = 0
				if c.socksServer != nil {
					message.ErrorStatus = 1
				}
				log.Printf("Starting socks server")
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

// Where the magic happens
func main() {
	var err error
	var cancel context.CancelFunc

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
		grpc.WithPerRPCCredentials(common.NewToken(clientToken)))

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

	gClient.grpcClient = cs.NewClientServiceClient(conn)
	gClient.gCtx, cancel = context.WithCancel(context.Background())
	defer cancel()

	emptyMsg := new(cs.Empty)
	configMsg, err := gClient.grpcClient.GetConfigurationMessage(gClient.gCtx, emptyMsg)
	if err != nil {
		return
	}

	gClient.endpoint.SetID(configMsg.EndpointID)

	conMsg := new(cs.EndpointControlMessage)
	conMsg.EndpointID = gClient.endpoint.Id
	gClient.ctrlStream, err = gClient.grpcClient.CreateEndpointControlStream(gClient.gCtx, conMsg)

	if err != nil {
		return
	}

	go gClient.receiveClientControlMessages()
	<-gClient.killClient
}
