package main

import (
	"flag"

	"github.com/hotnops/gTunnel/gserver/gserverlib"
)

var (
	tls        = flag.Bool("tls", true, "Connection uses TLS if true, else plain HTTP")
	certFile   = flag.String("cert_file", "tls/cert", "The TLS cert file")
	keyFile    = flag.String("key_file", "tls/key", "The TLS key file")
	clientPort = flag.Int("clientPort", 443, "The server port")
	adminPort  = flag.Int("adminPort", 1337, "The server port")
)

// What it do
func main() {
	flag.Parse()

	s := gserverlib.NewGServer()
	s.Start(*clientPort, *adminPort, *tls, *certFile, *keyFile)

}
