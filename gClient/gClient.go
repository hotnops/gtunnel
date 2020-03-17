package main

import (
	"context"
	"flag"
	"gTunnel/common"
	pb "gTunnel/gTunnel"
	"io"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/testdata"
)

var (
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "", "The file containing the CA root cert file")
	serverAddr         = flag.String("server_addr", "127.0.0.1:5555", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake")
)

func intToIP(ip uint32) string {
	result := make(net.IP, 4)
	result[3] = byte(ip)
	result[2] = byte(ip >> 8)
	result[1] = byte(ip >> 16)
	result[0] = byte(ip >> 24)
	return result.String()
}

type gClient struct {
	endpoint   *common.Endpoint
	ctrlStream pb.GTunnel_CreateEndpointControlStreamClient
	grpcClient pb.GTunnelClient
	killClient chan bool
	gCtx       context.Context
}

type ClientStreamHandler struct {
	client pb.GTunnelClient
	gCtx   context.Context
}

func (c *ClientStreamHandler) GetByteStream(connId int32) common.ByteStream {
	stream, err := c.client.CreateConnectionStream(c.gCtx)
	if err != nil {
		log.Printf("Failed to get byte stream: %v", err)
	}

	return stream
}

func (c *ClientStreamHandler) CloseStream(connId int32) {

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
				log.Fatalf("%v = %v", c, err)
			}
			ctrlMessageChan <- message
		}
	}(c.ctrlStream)

	for {
		select {
		case message := <-ctrlMessageChan:
			log.Printf("Got Endpoint control message")
			operation := message.Operation
			if operation == common.EndpointCtrlAddTunnel {
				log.Println("Adding new tunnel")

				newTunnel := common.NewTunnel(message.TunnelID, message.LocalIp, message.LocalPort, message.RemoteIP, message.RemotePort)
				f := new(ClientStreamHandler)
				f.client = c.grpcClient
				f.gCtx = c.gCtx
				newTunnel.ConnectionHandler = f

				tStream, _ := c.grpcClient.CreateTunnelControlStream(c.gCtx)

				newTunnel.SetControlStream(tStream)
				tMsg := new(pb.TunnelControlMessage)
				tMsg.EndpointID = message.EndpointID
				tMsg.TunnelID = message.TunnelID
				tStream.Send(tMsg)

				c.endpoint.AddTunnel(message.TunnelID, newTunnel)
				newTunnel.Start()

			}

		case <-c.killClient:
			log.Printf("Client killed.")
			return
		}
	}
}

func main() {
	flag.Parse()
	var err error
	var cancel context.CancelFunc

	var opts []grpc.DialOption
	if *tls {
		if *caFile == "" {
			*caFile = testdata.Path("ca.pem")
		}
	}
	opts = append(opts, grpc.WithInsecure())

	gClient := new(gClient)
	gClient.endpoint = common.NewEndpoint("aaaaa")
	gClient.killClient = make(chan bool)

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	gClient.grpcClient = pb.NewGTunnelClient(conn)
	gClient.gCtx, cancel = context.WithCancel(context.Background())
	defer cancel()

	conMsg := new(pb.EndpointControlMessage)
	conMsg.EndpointID = gClient.endpoint.Id
	gClient.ctrlStream, err = gClient.grpcClient.CreateEndpointControlStream(gClient.gCtx, conMsg)

	if err != nil {
		log.Fatalf("GetEndpointMessage failed: %v", err)
	}

	go gClient.receiveClientControlMessages()
	<-gClient.killClient
}
