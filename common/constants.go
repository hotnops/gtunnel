package common

const (
	EndpointCtrlDisconnect = iota
	EndpointCtrlAddTunnel
	EndpointCtrlDeleteTunnel
)

const (
	TunnelCtrlConnect = iota
	TunnelCtrlAck
	TunnelCtrlDisconnect
)
