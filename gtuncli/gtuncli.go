package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/hotnops/gTunnel/common"
	as "github.com/hotnops/gTunnel/grpc/admin"
	"github.com/olekukonko/tablewriter"
	"google.golang.org/grpc"
)

// ServerHost constant is the env variable
// used to configure the host for the gtunnel server
const ServerHost = "GTUNNEL_HOST"

// ServerPort constant is the env variable
// used to configure the port for the gtunnel server
const ServerPort = "GTUNNEL_PORT"

// ConfigFileName is the filename in which
// configuration parameters will be read
const ConfigFileName = ".gtunnel.conf"

var commands = []string{
	"clientlist",
	"clientregister",
	"clientdisconnect",
	"tunnelcreate",
	"tunneldelete",
	"tunnellist",
	"connectionlist",
	"socksstart",
	"socksstop",
	"help"}

func printCommands(progName string) {
	fmt.Printf("[*] Available commands: \n")

	for _, command := range commands {
		fmt.Printf("\t%s\n", command)
	}
}

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
	table.SetHeader([]string{"Name", "Unique ID", "Status", "Remote Address", "Hostname", "Date Connected"})
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("[!] Error receiving: %s", err)
		} else {
			name := message.Name
			status := fmt.Sprintf("%d", message.Status)
			row := []string{name,
				message.ClientId,
				status,
				message.RemoteAddress,
				message.Hostname,
				message.ConnectDate}
			table.Append(row)
		}
	}
	table.Render()
}

func clientRegister(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {
	clientCreateCmd := flag.NewFlagSet(commands[1], flag.ExitOnError)
	clientPlatform := clientCreateCmd.String("platform", "",
		"The operating system platform")
	serverIP := clientCreateCmd.String("ip", "",
		"Address to which the client will connect.")
	serverPort := clientCreateCmd.Int("port", 443,
		"The port to which the client will connect")
	name := clientCreateCmd.String("name", "",
		"The unique ID for the generated client. Can be a friendly name")
	token := clientCreateCmd.String("token", "", "The token used for authentication")
	binType := clientCreateCmd.String("bintype", "",
		"The type of output file. Options are exe or dll. Exe works on linux.")

	arch := clientCreateCmd.String("arch", "",
		"The architecture of the binary. Options are x64 or x64")

	proxyServer := clientCreateCmd.String("proxy", "", "A proxy server that the client will call through. Empty by default")

	clientCreateCmd.Parse(args)

	if *clientPlatform == "" {
		fmt.Println("[!] clientregister failed: platform required")
		return
	}
	if *serverIP == "" {
		fmt.Println("[!] clientregister failed: ip required")
		return
	}
	if *name == "" {
		fmt.Println("[!] clientregister failed: name required")
		return
	}
	if *token == "" {
		fmt.Println("[!] clientregister failed: token required")
		return
	}
	if *arch == "" {
		fmt.Println("[!] clientregister failed: arch required")
		return
	}

	ip := net.ParseIP(*serverIP)

	clientCreateReq := new(as.ClientRegisterRequest)
	clientCreateReq.ClientId = *name
	clientCreateReq.IpAddress = common.IpToInt32(ip)
	clientCreateReq.Port = uint32(*serverPort)
	clientCreateReq.Platform = *clientPlatform
	clientCreateReq.BinType = *binType
	clientCreateReq.Arch = *arch
	clientCreateReq.ProxyServer = *proxyServer
	clientCreateReq.Token = *token

	resp, err := adminClient.ClientRegister(ctx, clientCreateReq)

	if err != nil {
		fmt.Printf("[!] ClientRegister failed: %s\n", err.Error())
	}
	if resp.Error != "" {
		fmt.Printf("[!] ClientRegister returned an error: %s\n", err.Error())
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
	disconnectReq.ClientId = *clientID

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
	tunnel.DestinationIp = common.IpToInt32(dIP)
	tunnel.DestinationPort = uint32(*destinationPort)
	tunnel.ListenIp = common.IpToInt32(lIP)
	tunnel.ListenPort = uint32(*listenPort)

	if len(*tunnelID) == 0 {
		tunnel.Id = common.GenerateString(common.TunnelIDSize)
	} else {
		tunnel.Id = *tunnelID
	}

	tunnelAddReq.ClientId = *clientID
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
	clientID := tunnelDeleteCmd.String("clientid", "",
		"The ID of the client that has the tunnel to be deleted")
	tunnelID := tunnelDeleteCmd.String("tunnelid", "",
		"The ID of the tunnel to delete")

	tunnelDeleteCmd.Parse(args)

	req := new(as.TunnelDeleteRequest)

	req.ClientId = *clientID
	req.TunnelId = *tunnelID

	_, err := adminClient.TunnelDelete(ctx, req)

	if err != nil {
		log.Fatalf("[!] Failed to delete tunnel: %s", err)
	}
}

func tunnelList(ctx context.Context,
	adminClient as.AdminServiceClient,
	args []string) {

	tunnelListCmd := flag.NewFlagSet(commands[5], flag.ExitOnError)
	clientID := tunnelListCmd.String("clientid", "",
		"Tunnels will be listed for this client ID")

	tunnelListCmd.Parse(args)
	req := new(as.TunnelListRequest)
	req.ClientId = *clientID

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

			listenIP := common.Int32ToIP(message.ListenIp)
			destIP := common.Int32ToIP(message.DestinationIp)
			listenPort := fmt.Sprintf("%d", message.ListenPort)
			destPort := fmt.Sprintf("%d", message.DestinationPort)

			row := []string{*clientID,
				message.Id,
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
	clientID := connectionListCmd.String("clientid", "",
		"The client for which connections will be listed")
	tunnelID := connectionListCmd.String("tunnelid", "",
		"The tunnel for which connections will be listed")

	connectionListCmd.Parse(args)
	req := new(as.ConnectionListRequest)
	req.ClientId = *clientID
	req.TunnelId = *tunnelID

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

			sourceIP := common.Int32ToIP(message.SourceIp)
			destIP := common.Int32ToIP(message.DestinationIp)

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
	clientID := socksStartCmd.String("clientid", "",
		"The ID of the client")
	socksPort := socksStartCmd.Int("port", 0,
		"The port on which to start the socks server")

	socksStartCmd.Parse(args)

	req := new(as.SocksStartRequest)
	req.ClientId = *clientID
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
	clientID := socksStopCmd.String("clientid", "",
		"The ID of the client")

	socksStopCmd.Parse(args)

	req := new(as.SocksStopRequest)
	req.ClientId = *clientID

	_, err := adminClient.SocksStop(ctx, req)

	if err != nil {
		log.Fatalf("[!] Failed to start socks server: %s", err)
	}
}

func loadConfiguration(hostname *string, port *int) {
	var configData map[string]interface{}

	data, err := ioutil.ReadFile(ConfigFileName)
	if err != nil {
		log.Printf("[*] No configuration file found. Using environment variables")
		return
	}

	err = json.Unmarshal([]byte(data), &configData)

	if err != nil {
		log.Printf("[!] Failed to desrialize configuration file")
		os.Exit(1)
	}

	if val, ok := configData["host"]; ok {
		*hostname = val.(string)
	}

	if val, ok := configData["port"]; ok {
		*port = int(val.(float64))
	}
}

func main() {

	/*
		connectionListCmd := flag.NewFlagSet(commands[5], flag.ExitOnError)

		socksStopCmd := flag.NewFlagSet(commands[7], flag.ExitOnError)
	*/

	if len(os.Args) == 1 {
		printCommands(os.Args[0])
		os.Exit(1)
	}

	if os.Args[1] == "-h" {
		printCommands(os.Args[0])
		os.Exit(1)
	}

	if os.Args[1] == "help" {
		printCommands(os.Args[0])
		os.Exit(1)
	}

	host := ""
	port := 0

	loadConfiguration(&host, &port)

	// Environment variables override configuration file
	if os.Getenv(ServerHost) != "" {
		host = os.Getenv(ServerHost)
	}
	if os.Getenv(ServerPort) != "" {
		var err error
		port, err = strconv.Atoi(os.Getenv(ServerPort))
		if err != nil {
			fmt.Println("[!] Invalid port specified.")
			os.Exit(1)
		}
	}

	if host == "" {
		fmt.Println("[!] No server host specified.")
		os.Exit(1)
	}

	if port == 0 {
		fmt.Println("[*] Defaulting port to 1337")
		port = 1337
	}

	adminClient, err := connect(host, uint32(port))

	if err != nil {
		log.Fatalf("[!] Failed to connect to server: %s", err)
	}

	ctx, _ := context.WithCancel(context.Background())

	switch os.Args[1] {
	case commands[0]:
		clientList(ctx, adminClient)
	// List out all the configured clients and their connection status
	case commands[1]:
		clientRegister(ctx, adminClient, os.Args[2:])
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
	case commands[9]:
		printCommands(os.Args[0])
		os.Exit(1)
	default:
		log.Printf("[*] Command: %s not recognized\n", os.Args[1])
	}

}
