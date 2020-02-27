package models

type BLEClientListener interface {
	OnConnected(string, int)
	OnDisconnected()
	OnTimeSync()
	OnInternalError(error)
}

type BLEServerStatusListener interface {
	OnServerStatusChanged(BLEServerStatus, error)
	OnClientStateMapChanged(*ConnectionGraph, *RssiMap, map[string]BLEClientState)
	OnClientLog(ClientLogRequest)
	OnInternalError(error)
}

type BLEForwarderListener interface {
	OnConnected(string, int)
	OnDisconnected()
	OnTimeSync()
	OnInternalError(error)
	OnServerStatusChanged(BLEServerStatus, error)
}
