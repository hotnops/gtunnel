package common

const (
	EndpointCtrlDisconnect = iota
	EndpointCtrlAddRTunnel
	EndpointCtrlAddTunnel
	EndpointCtrlSocksProxy
	EndpointCtrlSocksProxyAck
	EndpointCtrlSocksKill
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
