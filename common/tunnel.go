package common

import (
	"fmt"
	pb "gTunnel/gTunnel"
	"log"
	"net"
)

type ConnectionStreamHandler interface {
	GetByteStream(connId int32) ByteStream
	CloseStream(connId int32)
}

type Tunnel struct {
	id                string
	localPort         uint32
	localIP           uint32
	remotePort        uint32
	remoteIP          uint32
	connections       map[int32]*Connection
	listeners         []net.Listener
	Kill              chan bool
	ctrlStream        TunnelControlStream
	connected         chan bool
	connectionCount   int32
	ConnectionHandler ConnectionStreamHandler
}

func NewTunnel(id string, localPort uint32, localIP uint32, remoteIP uint32, remotePort uint32) *Tunnel {
	t := new(Tunnel)
	t.id = id
	t.localIP = localIP
	t.localPort = localPort
	t.remoteIP = remoteIP
	t.remotePort = remotePort
	t.connections = make(map[int32]*Connection)
	t.Kill = make(chan bool)
	t.connected = make(chan bool)
	t.listeners = make([]net.Listener, 0)
	return t
}

func (t *Tunnel) GetConnection(connId int32) *Connection {
	if conn, ok := t.connections[connId]; ok {
		return conn
	} else {
		log.Printf("Attempted to get connection that doesn't exist: %d", connId)
		return nil
	}
}

func (t *Tunnel) GetConnections() map[int32]*Connection {
	return t.connections
}

func (t *Tunnel) SignalConnect() {
	t.connected <- true
}

func (t *Tunnel) GetControlStream() TunnelControlStream {
	return t.ctrlStream
}

func (t *Tunnel) SetControlStream(s TunnelControlStream) {
	t.ctrlStream = s
}

func (t *Tunnel) Start() {
	// A thread for handling the established tcp connections
	go t.handleIngressCtrlMessages()

}

func (t *Tunnel) Stop() {
	// First, stop all the listeners
	for _, ln := range t.listeners {
		ln.Close()
	}

	// Close all existing tcp connections
	for _, conn := range t.connections {
		conn.Close()
	}
	// Lastly, signal that the tunnel stream should be killed
	t.Kill <- true
}

func (t *Tunnel) handleIngressCtrlMessages() {
	ingressMessages := make(chan *pb.TunnelControlMessage)
	go func(s TunnelControlStream) {
		for {
			ingressMessage, _ := t.ctrlStream.Recv()
			ingressMessages <- ingressMessage
		}
	}(t.ctrlStream)
	for {
		select {
		case ctrlMessage, ok := <-ingressMessages:
			if !ok {
				log.Printf("Failed to read from ingressMessages.")
				break
			}
			// handle control message
			if ctrlMessage.Operation == TunnelCtrlConnect {
				log.Println("Got tunnel ctrl connect message")
				log.Printf("Attempting to make new connection to %d:%d\n", t.remoteIP, t.remotePort)

				ctrlMessage.Operation = TunnelCtrlAck

				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", Int32ToIP(t.remoteIP), t.remotePort))
				if err != nil {
					log.Printf("Failed to connect to address specified by tunnel: %v ", err)
					ctrlMessage.ErrorStatus = 1
				} else {
					gConn := NewConnection(conn)
					stream := t.ConnectionHandler.GetByteStream(ctrlMessage.ConnectionID)

					bytesMessage := new(pb.BytesMessage)
					bytesMessage.EndpointID = ctrlMessage.EndpointID
					bytesMessage.TunnelID = ctrlMessage.TunnelID
					bytesMessage.ConnectionID = ctrlMessage.ConnectionID

					stream.Send(bytesMessage)
					gConn.SetStream(stream)
					gConn.Start()

					t.connections[ctrlMessage.ConnectionID] = gConn
				}
				t.ctrlStream.Send(ctrlMessage)
			} else if ctrlMessage.Operation == TunnelCtrlAck {
				log.Println("Got a tunnel ctrl Ack message")
				//conn := t.connections[ctrlMessage.ConnectionID]
				if ctrlMessage.ErrorStatus != 0 {
					log.Println("Failed to connect to remote IP. Deleting connection")
					t.RemoveConnection(ctrlMessage.ConnectionID)
				} else {
					// Now that we know we are connected, we need to create a new byte
					// stream and create a thread to service it
					conn, ok := t.connections[ctrlMessage.ConnectionID]
					if !ok {
						log.Printf("Got an ack for a non-existent connection: %d", ctrlMessage.ConnectionID)
					} else {
						log.Printf("Starting connection")
						// Waiting until the byte stream gets set up
						<-conn.Connected
						conn.Start()
					}
				}
			} else if ctrlMessage.Operation == TunnelCtrlDisconnect {
				log.Println("Got tunnel ctrl connect message")
				t.RemoveConnection(ctrlMessage.ConnectionID)
			}

		case <-t.Kill:
			break
		}
	}
}

func (t *Tunnel) AddListener(listenPort int32, endpointId string) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", listenPort))
	if err != nil {
		log.Printf("Failed to start listener on port %d : %v", listenPort, err)
		return false
	}

	t.listeners = append(t.listeners, ln)

	newConns := make(chan net.Conn)

	go func(l net.Listener) {
		for {
			c, err := l.Accept()
			if err == nil {
				newConns <- c
			}
		}
	}(ln)
	go func() {
		for {
			select {
			case conn := <-newConns:
				gConn := NewConnection(conn)
				t.AddConnection(gConn)
				log.Printf("Accepted new connection")
				newMessage := new(pb.TunnelControlMessage)
				newMessage.EndpointID = endpointId
				newMessage.Operation = TunnelCtrlConnect
				newMessage.TunnelID = t.id
				newMessage.ConnectionID = gConn.Id
				t.ctrlStream.Send(newMessage)

			case <-t.Kill:
				log.Printf("Tunnel closed. Stopping listener on port %d", listenPort)
				return
			}
		}
	}()
	return true
}

func (t *Tunnel) AddConnection(c *Connection) {
	c.Id = t.connectionCount
	t.connections[t.connectionCount] = c
	t.connectionCount += 1
}

func (t *Tunnel) RemoveConnection(connId int32) {
	delete(t.connections, connId)
}
