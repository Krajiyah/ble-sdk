package util

const (
	// MTU is used for the max size of bytes allowed for data transmittion between ble client and server
	MTU = 60
	// WriteForwardCharUUID represents UUID for ble characteristic which handles forwarding of writes
	WriteForwardCharUUID = "00030000-0003-1000-8000-00805F9B34FB"
	// StartReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads
	StartReadForwardCharUUID = "00030000-0004-1000-8000-00805F9B34FB"
	// EndReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads
	EndReadForwardCharUUID = "00030000-0006-1000-8000-00805F9B34FB"
	// ReadRssiMapCharUUID represents UUID for ble characteristic which handles forwarding of reads
	ReadRssiMapCharUUID = "00030000-0005-1000-8000-00805F9B34FB"
)
