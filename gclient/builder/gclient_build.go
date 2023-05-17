package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/hotnops/gTunnel/common"
	"golang.org/x/exp/slices"
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
	tokenOutputPath := outputPath + ".token"

	tokenFile, err := os.Create(tokenOutputPath)
	if err != nil {
		log.Printf("[!] Could not create token file")
		return err
	}

	if _, err := tokenFile.WriteString(token); err != nil {
		log.Printf("[!] Could not write token to file")
		return err
	}

	var loaderFlags = ""

	if platform == "win" {
		loaderFlags = "-extldflags \"-static\" -s -w -X main.clientToken=%s -X main.serverAddress=%s -X main.serverPort=%d -X main.httpsProxyServer=%s"
	} else {
		loaderFlags = "-s -w -X main.clientToken=%s -X main.serverAddress=%s -X main.serverPort=%d -X main.httpsProxyServer=%s"
	}

	flagString := fmt.Sprintf(loaderFlags, token, serverAddress, serverPort, proxyServer)
	var commands []string

	commands = append(commands, "build")
	env := os.Environ()
	target_files := []string{"gclient/gClient.go"}

	if binType == "lib" {
		commands = append(commands, "-buildmode=c-shared")
		env = append(env, "CGO_ENABLED=1")
		target_files = append(target_files, "gclient/cgo.go")
	}

	commands = append(commands, "-ldflags", flagString, "-o", outputPath)
	commands = append(commands, target_files...)

	if arch == "x86" {
		env = append(env, "GOARCH=386")
		if platform == "win" {
			env = append(env, "CC=i686-w64-mingw32-gcc")
		}
	} else if arch == "x64" {
		env = append(env, "GOARCH=amd64")
		if platform == "win" {
			env = append(env, "CC=x86_64-w64-mingw32-gcc")
		}
	} else {
		log.Fatal("[!] Invalid architecture: %s", arch)
	}

	if platform == "win" {
		env = append(env, "GOOS=windows")
	} else if platform == "linux" {
		env = append(env, "GOOS=linux")
	} else if platform == "mac" {
		env = append(env, "GOOS=darwin")
	} else {
		log.Printf("[!] Invalid platform")
		return nil
	}
	cmd := exec.Command("go", commands...)
	log.Printf("[*] Build cmd: %s\n", cmd.String())
	log.Printf("[*] Env: %s\n", env)

	cmd.Env = env
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
		"The type of output file. Options are exe or lib. Exe works on linux.")

	arch := flag.String("arch", "x64",
		"The architecture of the binary. Options are x86 or x64")

	proxyServer := flag.String("proxy", "", "A proxy server that the client will call through. Empty by default")

	flag.Parse()

	platforms := []string{"win", "mac", "linux"}
	bintypes := []string{"exe", "lib"}
	archs := []string{"x86", "x64"}

	if !slices.Contains(platforms, *platform) {
		fmt.Println(("[!] Invalid platform provided"))
		os.Exit(1)
	}

	if !slices.Contains(bintypes, *binType) {
		fmt.Println("[!] Invalid bintype")
		os.Exit(1)
	}

	if !slices.Contains(archs, *arch) {
		fmt.Println("[!] Invalid architecture")
		os.Exit(1)
	}

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
