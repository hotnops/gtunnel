package common

import (
	pb "gTunnel/gTunnel"
)

type TunnelControlStream interface {
	Send(*pb.TunnelControlMessage) error
	Recv() (*pb.TunnelControlMessage, error)
}

type ByteStream interface {
	Send(*pb.BytesMessage) error
	Recv() (*pb.BytesMessage, error)
}

type gInterface interface {
	GetTunnelStream() TunnelControlStream
	GetByteStream() ByteStream
}

type Endpoint struct {
	Id                 string
	killClient         chan bool
	tunnels            map[string]*Tunnel
	endpointCtrlStream chan pb.EndpointControlMessage
}

func (e *Endpoint) AddTunnel(id string, t *Tunnel) {
	e.tunnels[id] = t
}

func (e *Endpoint) GetTunnels() map[string]*Tunnel {
	return e.tunnels
}

func (e *Endpoint) GetTunnel(tunId string) *Tunnel {
	t, ok := e.tunnels[tunId]
	if ok {
		return t
	} else {
		return nil
	}
}

func NewEndpoint(id string) *Endpoint {
	e := new(Endpoint)
	e.Id = id
	e.endpointCtrlStream = make(chan pb.EndpointControlMessage)
	e.tunnels = make(map[string]*Tunnel)
	return e
}
