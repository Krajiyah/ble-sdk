package server

// BLEServerStatus is an enum for all possible status conditions for ble server
type BLEServerStatus int

const (
	// Running indicates ble server is healthy and running
	Running BLEServerStatus = iota
	// Crashed indicates ble server is not running and has returned error in execution
	Crashed
)
