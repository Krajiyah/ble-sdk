package models

// BLEClientStatus is an enum for all possible status conditions for ble client
type BLEClientStatus int

const (
	// Connected indicates ble client is healthy, and connected to server
	Connected BLEClientStatus = iota
	// Disconnected indicates ble client was connected at one point, but now is not connected to server
	Disconnected
)

func (s BLEClientStatus) String() string {
	return []string{"connected", "disconnected"}[s]
}

// BLEClientState is representation of ble client's state
type BLEClientState struct {
	Name          string
	Status        BLEClientStatus
	ConnectedAddr string
	RssiMap       map[string]map[string]int
}
