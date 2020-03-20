package main

import (
	"context"
	"fmt"
	"gTunnel/common"
	pb "gTunnel/gTunnel"
	"io"
	"google.golang.org/grpc"
)

var ID = "UNCONFIGURED"
var serverAddress = "UNCONFIGURED"
var serverPort = "" // This needs to be a string to be used with -X

type gClient struct {
	endpoint   *common.Endpoint
	ctrlStream pb.GTunnel_CreateEndpointControlStreamClient
	grpcClient pb.GTunnelClient
	killClient chan bool
	gCtx       context.Context
}

type ClientStreamHandler struct {
	client     pb.GTunnelClient
	gCtx       context.Context
	ctrlStream common.TunnelControlStream
}


func (c *ClientStreamHandler) GetByteStream(ctrlMessage *pb.TunnelControlMessage) common.ByteStream {
	stream, err := c.client.CreateConnectionStream(c.gCtx)
	if err != nil {
		return nil
	}

	// Once byte stream is open, send an initial message
	// with all the appropriate IDs
	bytesMessage := new(pb.BytesMessage)
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

func (c *ClientStreamHandler) Acknowledge(ctrlMessage *pb.TunnelControlMessage) common.ByteStream {
	return c.GetByteStream(ctrlMessage)
}

func (c *ClientStreamHandler) CloseStream(connId int32) {
	return
}

func (c *gClient) receiveClientControlMessages() {
	ctrlMessageChan := make(chan *pb.EndpointControlMessage)

	go func(c pb.GTunnel_CreateEndpointControlStreamClient) {
		for {
			message, err := c.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return
			}
			ctrlMessageChan <- message
		}
	}(c.ctrlStream)

	for {
		select {
		case message := <-ctrlMessageChan:
			operation := message.Operation
			if operation == common.EndpointCtrlAddTunnel {

				newTunnel := common.NewTunnel(message.TunnelID, message.LocalIp, message.LocalPort, message.RemoteIP, message.RemotePort)
				f := new(ClientStreamHandler)
				f.client = c.grpcClient
				f.gCtx = c.gCtx

				if message.LocalPort != 0 {
					newTunnel.AddListener(int32(message.LocalPort), c.endpoint.Id)
				}

				tStream, _ := c.grpcClient.CreateTunnelControlStream(c.gCtx)

				// Once we have the control stream, set it in our client handler
				f.ctrlStream = tStream
				newTunnel.ConnectionHandler = f
				newTunnel.SetControlStream(tStream)

				// Send a message through the new stream
				// to let the server know the ID specifics
				tMsg := new(pb.TunnelControlMessage)
				tMsg.EndpointID = message.EndpointID
				tMsg.TunnelID = message.TunnelID
				tStream.Send(tMsg)

				c.endpoint.AddTunnel(message.TunnelID, newTunnel)
				newTunnel.Start()

			} else if operation == common.EndpointCtrlDisconnect {
				close(c.killClient)
			}

		case <-c.killClient:
			return
		}
	}
}

func main() {
	var err error
	var cancel context.CancelFunc

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	//retryCount := 5
	//retryPeriod := 30

	gClient := new(gClient)
	gClient.endpoint = common.NewEndpoint(ID)
	gClient.killClient = make(chan bool)

	serverAddr := fmt.Sprintf("%s:%s", serverAddress, serverPort)

	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return
	}
	defer conn.Close()

	gClient.grpcClient = pb.NewGTunnelClient(conn)
	gClient.gCtx, cancel = context.WithCancel(context.Background())
	defer cancel()

	conMsg := new(pb.EndpointControlMessage)
	conMsg.EndpointID = gClient.endpoint.Id
	gClient.ctrlStream, err = gClient.grpcClient.CreateEndpointControlStream(gClient.gCtx, conMsg)

	if err != nil {
		return
	}

	go gClient.receiveClientControlMessages()
	<-gClient.killClient
}
