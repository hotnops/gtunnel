package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hotnops/gTunnel/gserver/gserverlib"
)

var (
	tls        = flag.Bool("tls", true, "Connection uses TLS if true, else plain HTTP")
	certFile   = flag.String("cert_file", "tls/cert", "The TLS cert file")
	keyFile    = flag.String("key_file", "tls/key", "The TLS key file")
	clientPort = flag.Int("clientPort", 443, "The server port")
	adminPort  = flag.Int("adminPort", 1337, "The server port")
	logfile    = flag.String("logFile", "", "The file where log output will be written")
)

// What it do
func main() {
	flag.Parse()

	var filePath = ""
	s := gserverlib.NewGServer()

	if *logfile == "" {
		time := strings.ReplaceAll(time.Now().UTC().String(), " ", "")
		filePath = fmt.Sprintf("logs/gtunnel_%s.log", time)
	} else {
		filePath = *logfile
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)

	if err != nil {
		log.Fatalf("[!] Failed to create log file.")
	}

	log.Printf("Logging output to : %s\n", file.Name())
	log.SetOutput(file)

	s.Start(*clientPort, *adminPort, *tls, *certFile, *keyFile)

}
