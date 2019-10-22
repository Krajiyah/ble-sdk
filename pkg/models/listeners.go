package models

// BLEServerStatusListener is an struct which can be used to implement custom state change listeners for server or for clients
type BLEServerStatusListener interface {
	OnServerStatusChanged(BLEServerStatus, error)
	OnClientStateMapChanged(map[string]BLEClientState)
	OnClientLog(ClientLogRequest)
	OnReadOrWriteError(error)
}
