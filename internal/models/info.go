package models

// Info is the metadata pushed by the Socket.IO "info" event.
type Info struct {
	Version        string `json:"version"`
	Timezone       string `json:"timezone"`
	TimezoneOffset string `json:"timezoneOffset"`
}
