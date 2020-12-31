package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/hotnops/gTunnel/common"
	as "github.com/hotnops/gTunnel/grpc/admin"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/grpc"
)

var commands = []string{
	"clientlist",
	"clientcreate",
	"clientdisconnect",
	"tunneladd",
	"tunneldelete",
	"tunnellist",
	"connectionlist",
	"socksstart",
	"socksstop"}

func connect(ip string, port uint32) (as.AdminServiceClient, error) {
	addr := fmt.Sprintf("%s:%d", ip, port)

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}

	adminClient := as.NewAdminServiceClient(conn)
	return adminClient, nil
}

func clientList(ctx context.Context, adminClient as.AdminServiceClient) {
	req := new(as.ClientListRequest)
	stream, err := adminClient.ClientList(ctx, req)
	if err != nil {
		log.Fatalf("[!] ClientList failed: %s", err)
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Status", "IP Address", "Port"})
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("[!] Error receiving: %s", err)
		} else {
			ip := common.Int32ToIP(message.IpAddress)
			status := fmt.Sprintf("%d", message.Status)
			port := fmt.Sprintf("%d", message.Port)
			row := []string{message.ClientID,
				status,
				ip.String(),
				port}
			table.Append(row)
		}
	}
	table.Render()
}

func clientCreate(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {
	clientCreateCmd := flag.NewFlagSet(commands[1], flag.ExitOnError)
	clientPlatform := clientCreateCmd.String("platform", "win",
		"The operating system platform")
	serverAddress := clientCreateCmd.String("serverAddress", "",
		"Address to which the client will connect.")
	serverPort := clientCreateCmd.Int("serverPort", 0,
		"The port to which the client will connect")
	clientID := clientCreateCmd.String("clientID", "",
		"The unique ID for the generated client. Can be a friendly name")
	outputFile := clientCreateCmd.String("outputFile", "",
		"The output file where the client binary will be written")

	clientCreateCmd.Parse(args)

	ip := net.ParseIP(*serverAddress)

	clientCreateReq := new(as.ClientCreateRequest)
	clientCreateReq.ClientID = *clientID
	clientCreateReq.IpAddress = common.IpToInt32(ip)
	clientCreateReq.Port = uint32(*serverPort)
	clientCreateReq.Platform = *clientPlatform
	// exeType is ignored for now
	clientCreateReq.ExeType = "ignored"
	stream, err := adminClient.ClientCreate(ctx, clientCreateReq)
	if err != nil {
		log.Fatalf("[!] ClientCreate failed: %s", err)
	}

	file, err := os.OpenFile(*outputFile, os.O_CREATE|os.O_RDWR, 0755)
	defer file.Close()

	if err != nil {
		log.Fatalf("[!] Create file failed: %s", err)
	}

	for {
		bytes, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("[!] Error receiving file: %s", err)
		} else {
			file.Write(bytes.Data)
		}
	}
}

func clientDisconnect(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	disconnectCmd := flag.NewFlagSet(commands[2], flag.ExitOnError)
	clientID := disconnectCmd.String("clientid", "",
		"The client to disconnect")
	disconnectCmd.Parse(args)

	disconnectReq := new(as.ClientDisconnectRequest)
	disconnectReq.ClientID = *clientID

	_, err := adminClient.ClientDisconnect(ctx, disconnectReq)
	if err != nil {
		log.Fatalf("[!] Failed to disconnect: %s", err)
	}
}

func tunnelAdd(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	tunnelAddCmd := flag.NewFlagSet(commands[3], flag.ExitOnError)
	clientID := tunnelAddCmd.String("clientid", "",
		"The ID of the client that will get the new tunnel")
	direction := tunnelAddCmd.String("direction", "forward",
		"The direction of the tunnel")
	listenIP := tunnelAddCmd.String("listenip", "0.0.0.0",
		"The IP address on which the listen port will bind to")
	listenPort := tunnelAddCmd.Int("listenport", 0,
		"The port on which to accept connections.")
	destinationIP := tunnelAddCmd.String("destinationip", "",
		"The IP to which connections will be forwarded")
	destinationPort := tunnelAddCmd.Int("destinationport", 0,
		"The port to which the connection will be forwarded")
	tunnelID := tunnelAddCmd.String("tunnelid", "",
		"A friendly name for the tunnel. A random string will be generated if none is provided")

	tunnelAddCmd.Parse(args)

	tunnelAddReq := new(as.TunnelAddRequest)
	tunnel := new(as.Tunnel)
	if *direction == "forward" {
		tunnel.Direction = common.TunnelDirectionForward
	} else if *direction == "reverse" {
		tunnel.Direction = common.TunnelDirectionReverse
	} else {
		log.Fatalf("Invalid direction. Should be 'forward' or 'reverse'")
	}
	lIP := net.ParseIP(*listenIP)
	dIP := net.ParseIP(*destinationIP)
	tunnel.DestinationIP = common.IpToInt32(dIP)
	tunnel.DestinationPort = uint32(*destinationPort)
	tunnel.ListenIP = common.IpToInt32(lIP)
	tunnel.ListenPort = uint32(*listenPort)

	if len(*tunnelID) == 0 {
		tunnel.ID = common.GenerateString(common.TunnelIDSize)
	} else {
		tunnel.ID = *tunnelID
	}

	tunnelAddReq.ClientID = *clientID
	tunnelAddReq.Tunnel = tunnel

	_, err := adminClient.TunnelAdd(ctx, tunnelAddReq)

	if err != nil {
		log.Fatalf("[!] TunnelAdd failed: %s", err)
	}

}

func tunnelDelete(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	tunnelDeleteCmd := flag.NewFlagSet(commands[4], flag.ExitOnError)
	clientID := tunnelDeleteCmd.String("clientID", "",
		"The ID of the client that has the tunnel to be deleted")
	tunnelID := tunnelDeleteCmd.String("tunnelID", "",
		"The ID of the tunnel to delete")

	tunnelDeleteCmd.Parse(args)

	req := new(as.TunnelDeleteRequest)

	req.ClientID = *clientID
	req.TunnelID = *tunnelID

	_, err := adminClient.TunnelDelete(ctx, req)

	if err != nil {
		log.Fatalf("[!] Failed to delete tunnel: %s", err)
	}
}

func tunnelList(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	tunnelListCmd := flag.NewFlagSet(commands[5], flag.ExitOnError)
	clientID := tunnelListCmd.String("clientID", "",
		"Tunnels will be listed for this client ID")

	tunnelListCmd.Parse(args)
	req := new(as.TunnelListRequest)
	req.ClientID = *clientID

	stream, err := adminClient.TunnelList(ctx, req)
	if err != nil {
		log.Fatalf("[!] TunnelList failed: %s", err)
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Client ID",
		"Tunnel ID",
		"Direction",
		"Listen IP",
		"Listen Port",
		"Destination IP",
		"Destination Port"})

	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("[!] Error receiving: %s", err)
		} else {
			var direction = ""
			if message.Direction == common.TunnelDirectionForward {
				direction = "forward"
			} else if message.Direction == common.TunnelDirectionReverse {
				direction = "reverse"
			}

			listenIP := common.Int32ToIP(message.ListenIP)
			destIP := common.Int32ToIP(message.DestinationIP)
			listenPort := fmt.Sprintf("%d", message.ListenPort)
			destPort := fmt.Sprintf("%d", message.DestinationPort)

			row := []string{*clientID,
				message.ID,
				direction,
				listenIP.String(),
				listenPort,
				destIP.String(),
				destPort}
			table.Append(row)

		}
	}

	table.Render()
}

func connectionList(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	connectionListCmd := flag.NewFlagSet(commands[6], flag.ExitOnError)
	clientID := connectionListCmd.String("clientID", "",
		"The client for which connections will be listed")
	tunnelID := connectionListCmd.String("tunnelID", "",
		"The tunnel for which connections will be listed")

	connectionListCmd.Parse(args)
	req := new(as.ConnectionListRequest)
	req.ClientID = *clientID
	req.TunnelID = *tunnelID

	stream, err := adminClient.ConnectionList(ctx, req)
	if err != nil {
		log.Fatalf("[!] ConnectionList failed: %s", err)
	}
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("[!] Error receiving: %s", err)
		} else {

			sourceIP := common.Int32ToIP(message.SourceIP)
			destIP := common.Int32ToIP(message.DestinationIP)

			log.Printf("%s\t%d\t%s\t%d\n",
				sourceIP,
				message.SourcePort,
				destIP,
				message.DestinationPort)
		}
	}
}

func socksStart(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	socksStartCmd := flag.NewFlagSet(commands[6], flag.ExitOnError)
	clientID := socksStartCmd.String("clientID", "",
		"The ID of the client")
	socksPort := socksStartCmd.Int("socksPort", 0,
		"The port on which to start the socks server")

	socksStartCmd.Parse(args)

	req := new(as.SocksStartRequest)
	req.ClientID = *clientID
	req.SocksPort = uint32(*socksPort)

	_, err := adminClient.SocksStart(ctx, req)

	if err != nil {
		log.Fatalf("[!] Failed to start socks server: %s", err)
	}
}

func socksStop(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	socksStopCmd := flag.NewFlagSet(commands[6], flag.ExitOnError)
	clientID := socksStopCmd.String("clientID", "",
		"The ID of the client")

	socksStopCmd.Parse(args)

	req := new(as.SocksStopRequest)
	req.ClientID = *clientID

	_, err := adminClient.SocksStop(ctx, req)

	if err != nil {
		log.Fatalf("[!] Failed to start socks server: %s", err)
	}
}

func main() {

	/*



		connectionListCmd := flag.NewFlagSet(commands[5], flag.ExitOnError)

		socksStopCmd := flag.NewFlagSet(commands[7], flag.ExitOnError)
	*/

	adminClient, err := connect("127.0.0.1", 1337)

	if err != nil {
		log.Fatalf("[!] Failed to connect to server: %s", err)
	}

	ctx, _ := context.WithCancel(context.Background())

	switch os.Args[1] {
	case commands[0]:
		clientList(ctx, adminClient)
	// List out all the configured clients and their connection status
	case commands[1]:
		clientCreate(ctx, adminClient, os.Args[2:])
	case commands[2]:
		clientDisconnect(ctx, adminClient, os.Args[2:])
	case commands[3]:
		tunnelAdd(ctx, adminClient, os.Args[2:])
	case commands[4]:
		tunnelDelete(ctx, adminClient, os.Args[2:])
	case commands[5]:
		tunnelList(ctx, adminClient, os.Args[2:])
	case commands[6]:
		connectionList(ctx, adminClient, os.Args[2:])
	case commands[7]:
		socksStart(ctx, adminClient, os.Args[2:])
	case commands[8]:
		socksStop(ctx, adminClient, os.Args[2:])
		/*


			case commands[5]:
				// connectionlist
				clientID := connectionListCmd.String("clientID", "",
					"The ID of the client")
				tunnelID := connectionListCmd.String("tunnelID", "",
					"The ID of the tunnel")

				connectionListCmd.Parse(os.Args[2:])

			case commands[6]:
				// socksstart


			case commands[7]:
				// socksstop
				clientID := socksStopCmd.String("clientID", "",
					"The ID of the client")
				socksStopCmd.Parse(os.Args[2:])
		*/
	}

}
