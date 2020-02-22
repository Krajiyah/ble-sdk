package models

// BLEServerStatusListener is an struct which can be used to implement custom state change listeners for server or for clients
type BLEServerStatusListener interface {
	OnServerStatusChanged(BLEServerStatus, error)
	OnClientStateMapChanged(*ConnectionGraph, *RssiMap, map[string]BLEClientState)
	OnClientLog(ClientLogRequest)
	OnReadOrWriteError(error)
}

// BLEForwarderListener is an struct which can be used to implement custom state change listeners for forwarders
type BLEForwarderListener interface {
	OnClientConnected(string, int, int)
	OnClientDisconnected()
	OnServerStatusChanged(BLEServerStatus, error)
	OnConnectionError(error)
	OnReadOrWriteError(error)
	OnError(error)
}
