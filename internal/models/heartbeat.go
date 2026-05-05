package models

// Heartbeat represents a single heartbeat record as emitted by the
// "heartbeat", "heartbeatList", and "importantHeartbeatList" events.
type Heartbeat struct {
	MonitorID int           `json:"monitorID"`
	Status    MonitorStatus `json:"status"`
	Time      string        `json:"time"`   // ISO 8601 timestamp
	Msg       string        `json:"msg"`
	Important bool          `json:"important"`
	Duration  int           `json:"duration"` // milliseconds
	Ping      int           `json:"ping"`     // milliseconds, -1 if unavailable
	DownCount int           `json:"downCount,omitempty"`
}

// HeartbeatList is emitted as ["heartbeatList", monitorID, [heartbeats], overwrite]
// The SDK unpacks this into HeartbeatListEvent.
type HeartbeatListEvent struct {
	MonitorID int
	Beats     []Heartbeat
	Overwrite bool
}
