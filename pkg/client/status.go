package client

// BLEClientStatus is an enum for all possible status conditions for ble client
type BLEClientStatus int

const (
	// Connected indicates ble client is healthy, and connected to server
	Connected BLEClientStatus = iota
	// Disconnected indicates ble client was connected at one point, but now is not connected to server
	Disconnected
)

func (s BLEClientStatus) String() string {
	return []string{"Connected", "Disconnected"}[s]
}

// BLEClientState is representation of ble client's state
type BLEClientState struct {
	Status  BLEClientStatus
	RssiMap map[string]int
}
