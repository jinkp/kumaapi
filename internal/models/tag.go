package models

// Tag represents an Uptime Kuma tag.
type Tag struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// MonitorTagRelation represents the relation row between a monitor and a tag.
type MonitorTagRelation struct {
	TagID     int    `json:"tag_id"`
	MonitorID int    `json:"monitor_id"`
	Value     string `json:"value"`
}
