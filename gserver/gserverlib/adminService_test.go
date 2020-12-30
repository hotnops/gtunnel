package gserverlib

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/hotnops/gTunnel/common"
	as "github.com/hotnops/gTunnel/grpc/admin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testClientPort = 8080
const testAdminPort = 1338
const clientID = "TESTID"

func setupClient(ctx context.Context, grpcServer as.AdminServiceClient) error {
	ip := net.ParseIP("127.0.0.1")

	req := new(as.ClientCreateRequest)
	req.ClientID = clientID
	req.IpAddress = common.IpToInt32(ip)
	req.Port = 8080
	req.Platform = "linux"
	req.ExeType = "elf"

	resp, err := grpcServer.ClientCreate(ctx, req)

	if err != nil {
		return fmt.Errorf("failed to create client")
	}

	binary := resp.ClientBinary

	if len(binary) == 0 {
		return fmt.Errorf("binary is empty")
	}

	return nil
}

func TestClientCommands(t *testing.T) {

	s := NewGServer()

	go s.Start(testClientPort, testAdminPort, false, "", "")

	log.Printf("connecting to server")
	serverAddress := fmt.Sprintf("0.0.0.0:%d", testAdminPort)

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	conn, err := grpc.Dial(serverAddress)

	for err != nil {
		log.Printf("%s", err)
		log.Printf("failed to connect to server. waiting 3 seconds")
		time.Sleep(3 * time.Second)
		conn, err = grpc.Dial(serverAddress, opts...)
	}

	log.Printf("connected")

	grpcClient := as.NewAdminServiceClient(conn)
	ctx, _ := context.WithCancel(context.Background())

	setupClient(ctx, grpcClient)

	t.Run("ClientListEmpty", func(t *testing.T) {
		clientListReq := new(as.ClientListRequest)
		respStream, err := grpcClient.ClientList(ctx, clientListReq)

		if err != nil {
			t.Fatalf("ClientList failed: %s", err)
		}

		for {
			_, err := respStream.Recv()
			if err == nil {
				t.Fatalf("error not raised on empty client set")
			}
			if e, ok := status.FromError(err); ok {
				if e.Code() != codes.OutOfRange {
					t.Fatalf("invalid error code returned")
				} else {
					break
				}
			} else {
				t.Fatalf("failed to get error code")
			}
		}
	})

	t.Run("ClientList", func(t *testing.T) {
		s.endpoints[clientID] = common.NewEndpoint()
		s.endpoints[clientID].SetID(clientID)
		clientListReq := new(as.ClientListRequest)
		respStream, err := grpcClient.ClientList(ctx, clientListReq)

		if err != nil {
			t.Fatalf("ClientList failed: %s", err)
		}

		for {
			message, err := respStream.Recv()
			if err == io.EOF {
				t.Fatalf("client not found")
			} else if err != nil {
				t.Fatalf("Failed to receive from stream")
			}
			if message.ClientID == clientID {
				break
			}
		}
	})

}
