package common

import (
	cs "github.com/hotnops/gTunnel/grpc/client"
)

type TunnelControlStream interface {
	Send(*cs.TunnelControlMessage) error
	Recv() (*cs.TunnelControlMessage, error)
}

type ByteStream interface {
	Send(*cs.BytesMessage) error
	Recv() (*cs.BytesMessage, error)
}

type gInterface interface {
	GetTunnelStream() TunnelControlStream
	GetByteStream() ByteStream
}

type Endpoint struct {
	Id                 string
	killClient         chan bool
	tunnels            map[string]*Tunnel
	endpointCtrlStream chan cs.EndpointControlMessage
}

// AddTunnel adds a tunnel instance to the list of tunnels
// maintained by the endpoint
func (e *Endpoint) AddTunnel(id string, t *Tunnel) {
	e.tunnels[id] = t
}

// GetTunnels returns all of the active tunnels
// maintained by the endpoint
func (e *Endpoint) GetTunnels() map[string]*Tunnel {
	return e.tunnels
}

//GetTunnel will take in a tunnel ID string as an argument
// and return a Tunnel pointer of the corresponding ID.
func (e *Endpoint) GetTunnel(tunID string) (*Tunnel, bool) {
	t, ok := e.tunnels[tunID]
	return t, ok
}

// StopAndDeleteTunnel takes in a tunnelID as an argument
// and removes the tunnel from the endpoint. Returns
// true if successful and false otherwise.
func (e *Endpoint) StopAndDeleteTunnel(tunID string) bool {
	tun, ok := e.tunnels[tunID]
	if !ok {
		return false
	}
	tun.Stop()
	delete(e.tunnels, tunID)
	return true
}

// Stop will close all tunnels and the associated TCP
// connections with each tunnel.
func (e *Endpoint) Stop() {
	for id, _ := range e.tunnels {
		e.StopAndDeleteTunnel(id)
	}
	close(e.endpointCtrlStream)
}

// NewEndpoint is a constructor for the endpoint
// class. It takes an endpoint ID string as an argument.
func NewEndpoint() *Endpoint {
	e := new(Endpoint)
	e.Id = ""
	e.endpointCtrlStream = make(chan cs.EndpointControlMessage)
	e.tunnels = make(map[string]*Tunnel)
	return e
}

// SetID takes in a string and will set the Id to that string
func (e *Endpoint) SetID(id string) {
	e.Id = id
}
