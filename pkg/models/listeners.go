package models

type BLEClientListener interface {
	OnConnected(string)
	OnDisconnected()
	OnTimeSync()
	OnInternalError(error)
}

type BLEServerStatusListener interface {
	OnClientStateMapChanged(*ConnectionGraph, *RssiMap, map[string]BLEClientState)
	OnClientLog(ClientLogRequest)
	OnInternalError(error)
}

type BLEForwarderListener interface {
	OnConnected(string)
	OnDisconnected()
	OnTimeSync()
	OnInternalError(error)
}
