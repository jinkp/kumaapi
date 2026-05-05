package models

import (
	"encoding/json"
	"fmt"
)

// FlexInt unmarshals a JSON value that can be either a number or a boolean.
// Uptime Kuma v2 is inconsistent: monitorList sends active=1/0 (int),
// while getMonitor sends active=true/false (bool).
type FlexInt int

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	// Try int first
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		*f = FlexInt(i)
		return nil
	}
	// Try bool
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		if b {
			*f = 1
		} else {
			*f = 0
		}
		return nil
	}
	return fmt.Errorf("models: FlexInt cannot unmarshal %s", string(data))
}

func (f FlexInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(f))
}

// IsActive returns true if the value represents an active/enabled state.
func (f FlexInt) IsActive() bool { return f != 0 }

func (f FlexInt) Int() int { return int(f) }
