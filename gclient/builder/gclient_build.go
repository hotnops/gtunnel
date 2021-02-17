package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/hotnops/gTunnel/common"
)

func GenerateClient(
	platform string,
	serverAddress string,
	serverPort uint16,
	clientID string,
	binType string,
	arch string,
	proxyServer string,
	outputFile string) error {

	token, err := common.GenerateToken()
	if err != nil {
		log.Printf("[!] Failed to generate token, I guess?")
		return err
	}

	outputPath := fmt.Sprintf("/output/%s", outputFile)

	flagString := fmt.Sprintf("-s -w -X main.clientToken=%s -X main.serverAddress=%s -X main.serverPort=%d -X main.httpsProxyServer=%s", token, serverAddress, serverPort, proxyServer)
	var commands []string

	commands = append(commands, "build")

	if binType == "lib" {
		commands = append(commands, "-buildmode=c-shared")
	}

	commands = append(commands, "-ldflags", flagString, "-o", outputPath, "gclient/gClient.go")

	cmd := exec.Command("go", commands...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	if platform == "win" {
		cmd.Env = append(cmd.Env, "GOOS=windows")
		if arch == "x86" {
			cmd.Env = append(cmd.Env, "CC=i686-w64-mingw32-gcc")
			cmd.Env = append(cmd.Env, "GOARCH=386")
		} else if arch == "x64" {
			cmd.Env = append(cmd.Env, "CC=x86_64-w64-mingw32-gcc")
			cmd.Env = append(cmd.Env, "GOARCH=amd64")
		}
	} else if platform == "linux" {
		cmd.Env = append(cmd.Env, "GOOS=linux")
		if arch == "x86" {
			cmd.Env = append(cmd.Env, "GOARCH=386")
		} else if arch == "x64" {
			cmd.Env = append(cmd.Env, "GOARCH=amd64")
		}
	}
	log.Printf("[*] Build cmd: %s\n", cmd.String())
	err = cmd.Run()
	if err != nil {
		log.Printf("[!] Failed to generate client: %s", err)
		return err
	}
	return nil
}

func main() {

	platform := flag.String("platform", "win",
		"The operating system platform")
	serverAddress := flag.String("ip", "",
		"Address to which the client will connect.")
	serverPort := flag.Int("port", 443,
		"The port to which the client will connect")
	clientID := flag.String("name", "",
		"The unique ID for the generated client. Can be a friendly name")
	outputFile := flag.String("outputfile", "",
		"The output file where the client binary will be written")
	binType := flag.String("bintype", "exe",
		"The type of output file. Options are exe or dll. Exe works on linux.")

	arch := flag.String("arch", "x64",
		"The architecture of the binary. Options are x64 or x64")

	proxyServer := flag.String("proxy", "", "A proxy server that the client will call through. Empty by default")

	flag.Parse()

	if *serverAddress == "" {
		fmt.Println("[!] ip not provided")
		os.Exit(1)
	}

	if *clientID == "" {
		fmt.Println("[!] name not provided")
		os.Exit(1)
	}

	if *outputFile == "" {
		fmt.Println("[!] outputFile not provided")
		os.Exit(1)
	}

	GenerateClient(
		*platform,
		*serverAddress,
		uint16(*serverPort),
		*clientID,
		*binType,
		*arch,
		*proxyServer,
		*outputFile)
}
