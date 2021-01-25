package common

import (
	"fmt"
	"log"
	"net"
	"sync"

	cs "github.com/hotnops/gTunnel/grpc/client"
	"github.com/segmentio/ksuid"
)

type ConnectionStreamHandler interface {
	GetByteStream(tunnel *Tunnel, ctrlMessage *cs.TunnelControlMessage) ByteStream
	CloseStream(tunnel *Tunnel, connID string)
	Acknowledge(tunnel *Tunnel, ctrlMessage *cs.TunnelControlMessage) ByteStream
}

type Tunnel struct {
	id                string
	direction         uint32
	listenIP          net.IP
	listenPort        uint32
	destinationIP     net.IP
	destinationPort   uint32
	connections       map[string]*Connection
	listeners         []net.TCPListener
	Kill              chan bool
	ctrlStream        TunnelControlStream
	ConnectionHandler ConnectionStreamHandler
	mutex             sync.Mutex
}

// NewTunnel is a constructor for the tunnel struct. It takes
// in id, direction, listenIP, lisetnPort, destinationIP, and
// destinationPort as parameters.
func NewTunnel(id string,
	direction uint32,
	listenIP net.IP,
	listenPort uint32,
	destinationIP net.IP,
	destinationPort uint32) *Tunnel {
	t := new(Tunnel)
	t.id = id
	t.direction = direction
	t.listenIP = listenIP
	t.listenPort = listenPort
	t.destinationIP = destinationIP
	t.destinationPort = destinationPort
	t.connections = make(map[string]*Connection)
	t.Kill = make(chan bool)
	t.listeners = make([]net.TCPListener, 0)
	return t
}

// Addconnection will generate an ID for the connection and
// add it to the map.
func (t *Tunnel) AddConnection(c *Connection) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	c.ID = ksuid.New().String()
	t.connections[c.ID] = c
}

// AddListener will start a tcp listener on a specific port and forward
// all accepted TCP connections to the associated tunnel.
func (t *Tunnel) AddListener(listenPort int32, clientID string) bool {

	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", listenPort))
	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return false
	}

	t.listeners = append(t.listeners, *ln)

	newConns := make(chan *net.TCPConn)

	go func(l *net.TCPListener) {
		for {
			c, err := l.AcceptTCP()
			if err == nil {
				newConns <- c
			} else {
				return
			}
		}
	}(ln)
	go func() {
		for {
			select {
			case conn := <-newConns:
				gConn := NewConnection(*conn)
				t.AddConnection(gConn)
				newMessage := new(cs.TunnelControlMessage)
				newMessage.Operation = TunnelCtrlConnect
				newMessage.TunnelId = t.id
				newMessage.ConnectionId = gConn.ID
				t.ctrlStream.Send(newMessage)

			case <-t.Kill:
				return
			}
		}
	}()
	return true
}

// GetConnection will return a Connection object
// with the given connection id
func (t *Tunnel) GetConnection(connID string) *Connection {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if conn, ok := t.connections[connID]; ok {
		return conn
	}
	return nil
}

// GetControlStream will return the control stream for
// the associated tunnel
func (t *Tunnel) GetControlStream() TunnelControlStream {
	return t.ctrlStream
}

// GetDestinationIP gets the destination IP of the tunnel.
func (t *Tunnel) GetDestinationIP() net.IP {
	return t.destinationIP
}

// GetDestinationPort gets the destination port of the tunnel.
func (t *Tunnel) GetDestinationPort() uint32 {
	return t.destinationPort
}

// GetDirection gets the direction of the tunnel (forward or reverse)
func (t *Tunnel) GetDirection() uint32 {
	return t.direction
}

// GetListenIP gets the ip address that the tunnel is listening on.
func (t *Tunnel) GetListenIP() net.IP {
	return t.listenIP
}

// GetListenPort gets the port that the tunnel is listening on.
func (t *Tunnel) GetListenPort() uint32 {
	return t.listenPort
}

// GetConnections will return the connection map
func (t *Tunnel) GetConnections() map[string]*Connection {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.connections
}

// handleIngressCtrlMessages is the loop function responsible
// for receiving control messages from the gRPC stream.
func (t *Tunnel) handleIngressCtrlMessages() {
	ingressMessages := make(chan *cs.TunnelControlMessage)
	go func(s TunnelControlStream) {
		for {
			ingressMessage, err := t.ctrlStream.Recv()
			if err != nil {
				close(ingressMessages)
				return
			}
			ingressMessages <- ingressMessage
		}
	}(t.ctrlStream)
	for {
		select {
		case ctrlMessage, ok := <-ingressMessages:
			if !ok {
				ingressMessages = nil
				break
			}
			if ctrlMessage == nil {
				ingressMessages = nil
				break
			}
			// handle control message
			if ctrlMessage.Operation == TunnelCtrlConnect {

				rAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d",
					t.destinationIP,
					t.destinationPort))

				conn, err := net.DialTCP("tcp", nil, rAddr)

				if err != nil {
					ctrlMessage.ErrorStatus = 1
				} else {
					var gConn *Connection
					if gConn, ok = t.connections[ctrlMessage.ConnectionId]; !ok {
						gConn = NewConnection(*conn)
						gConn.ID = ctrlMessage.ConnectionId
						t.connections[ctrlMessage.ConnectionId] = gConn
					}
					stream := t.ConnectionHandler.GetByteStream(t, ctrlMessage)
					gConn.SetStream(stream)
					gConn.Start()

				}

			} else if ctrlMessage.Operation == TunnelCtrlAck {
				if ctrlMessage.ErrorStatus != 0 {
					t.RemoveConnection(ctrlMessage.ConnectionId)
				} else {
					// Now that we know we are connected, we need to create a new byte
					// stream and create a thread to service it
					// If this is client side, we need to still create the byte stream
					conn := t.GetConnection(ctrlMessage.ConnectionId)

					if conn == nil {
						log.Printf("[!] Failed to get connection id: %d\n", ctrlMessage.ConnectionId)
					} else {
						// Waiting until the byte stream gets set up
						conn.SetStream(t.ConnectionHandler.Acknowledge(t, ctrlMessage))
						if ok {
							conn.Start()
						}
					}
				}
			} else if ctrlMessage.Operation == TunnelCtrlDisconnect {
				t.RemoveConnection(ctrlMessage.ConnectionId)
			}
		case <-t.Kill:
			break
		}
		if ingressMessages == nil {
			break
		}
	}
}

// RemoveConnection will remove the Connection object
// from the connections map.
func (t *Tunnel) RemoveConnection(connID string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.connections, connID)
}

// SetControlStream will set the provided control stream for
// the associated tunnel
func (t *Tunnel) SetControlStream(s TunnelControlStream) {
	t.ctrlStream = s
}

// Start receiving control messages for the tunnel
func (t *Tunnel) Start() {
	// A thread for handling the established tcp connections
	go t.handleIngressCtrlMessages()

}

// Stop will stop all associated goroutines for the tunnel
// and disconnect any associated TCP connections
func (t *Tunnel) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// First, stop all the listeners
	for _, ln := range t.listeners {
		ln.Close()
	}

	// Close all existing tcp connections
	for _, conn := range t.connections {
		conn.Close()
	}
	// Lastly, signal that the tunnel stream should be killed
	close(t.Kill)
}
