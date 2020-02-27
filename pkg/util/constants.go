package util

const (
	// MTU is used for the max size of bytes allowed for data transmittion between ble client and server
	MTU = 256
	// MainServiceUUID represents UUID for ble service for all ble characteristics
	MainServiceUUID = "00010000-0001-1000-8000-00805F9B34FB"
	// ClientStateUUID represents UUID for ble characteristic which handles writes from client to server for client state changes
	ClientStateUUID = "00010000-0003-1000-8000-00805F9B34FB"
	// TimeSyncUUID represents UUID for ble clients to time sync with ble server
	TimeSyncUUID = "00010000-0004-1000-8000-00805F9B34FB"
	// ClientLogUUID represents UUID for ble clients to write logs to
	ClientLogUUID = "00010000-0006-1000-8000-00805F9B34FB"
	// WriteForwardCharUUID represents UUID for ble characteristic which handles forwarding of writes
	WriteForwardCharUUID = "00030000-0003-1000-8000-00805F9B34FB"
	// StartReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads (start)
	StartReadForwardCharUUID = "00030000-0004-1000-8000-00805F9B34FB"
	// EndReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads (end)
	EndReadForwardCharUUID = "00030000-0006-1000-8000-00805F9B34FB"
	// ReadRssiMapCharUUID represents UUID for ble characteristic which is a getter for rssi map data
	ReadRssiMapCharUUID = "00030000-0005-1000-8000-00805F9B34FB"
	// ReadConnectionGraphUUID represents UUID for ble characteristic which is a getter for connection graph data
	ReadConnectionGraphUUID = "00030000-0007-1000-8000-00805F9B34FB"
)
