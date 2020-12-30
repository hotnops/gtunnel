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
	TunnelDirectionForward = iota
	TunnelDirectionReverse
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

const (
	ConfigurationFile = ".gtunnel.conf"
)
