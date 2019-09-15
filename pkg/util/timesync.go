package util

import (
	"time"
)

// TimeSync is a utility struct for syncing time with a given source of truth
type TimeSync struct {
	offset int64
	past   bool
}

// NewTimeSync generates the utility struct for time syncing
func NewTimeSync(initTS int64) TimeSync {
	now := UnixTS()
	t := TimeSync{}
	if initTS > now {
		t.offset = initTS - now
		t.past = true
	} else {
		t.offset = now - initTS
		t.past = false
	}
	return t
}

// TS returns the current unix (epoch) timestamp in milliseconds (offset by diff from source of truth)
func (t *TimeSync) TS() int64 {
	ts := UnixTS()
	if t.past {
		return t.offset + ts
	}
	return ts - t.offset
}

// UnixTS returns the current unix (epoch) timestamp in milliseconds
func UnixTS() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
