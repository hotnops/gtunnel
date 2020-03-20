package common

const (
	EndpointCtrlDisconnect = iota
	EndpointCtrlAddRTunnel
	EndpointCtrlAddTunnel
	EndpointCtrlDeleteTunnel
)

const (
	TunnelCtrlConnect = iota
	TunnelCtrlAck
	TunnelCtrlDisconnect
)

const (
	ConnectionStatusConnected = iota
	ConnectionStatusClosed
)
