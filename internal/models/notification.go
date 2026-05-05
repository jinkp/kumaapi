package models

import "encoding/json"

// Notification represents a notification provider configuration.
// Type-specific fields are stored in Config because the JSON shape varies
// across providers (telegram, slack, webhook, etc.).
type Notification struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Active    FlexInt        `json:"active"`
	IsDefault FlexInt        `json:"isDefault"`
	Config    map[string]any `json:"-"`
}

// AddNotificationRequest is a helper for building provider payloads.
// The Config map is merged into the root JSON object during marshaling.
type AddNotificationRequest struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Active    bool           `json:"active"`
	IsDefault bool           `json:"isDefault"`
	Config    map[string]any `json:"-"`
}

func (n *Notification) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	type alias Notification
	var base struct {
		ID        int     `json:"id"`
		Name      string  `json:"name"`
		Type      string  `json:"type"`
		Active    FlexInt `json:"active"`
		IsDefault FlexInt `json:"isDefault"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return err
	}

	n.ID = base.ID
	n.Name = base.Name
	n.Type = base.Type
	n.Active = base.Active
	n.IsDefault = base.IsDefault

	delete(raw, "id")
	delete(raw, "name")
	delete(raw, "type")
	delete(raw, "active")
	delete(raw, "isDefault")

	if len(raw) == 0 {
		n.Config = nil
		return nil
	}

	config := make(map[string]any, len(raw))
	for key, value := range raw {
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			return err
		}
		config[key] = decoded
	}
	n.Config = config
	return nil
}

func (r AddNotificationRequest) MarshalJSON() ([]byte, error) {
	payload := map[string]any{
		"name":      r.Name,
		"type":      r.Type,
		"active":    r.Active,
		"isDefault": r.IsDefault,
	}
	for key, value := range r.Config {
		payload[key] = value
	}
	return json.Marshal(payload)
}

// Payload returns the flattened root payload expected by Uptime Kuma.
func (r AddNotificationRequest) Payload() map[string]any {
	payload := map[string]any{
		"name":      r.Name,
		"type":      r.Type,
		"active":    r.Active,
		"isDefault": r.IsDefault,
	}
	for key, value := range r.Config {
		payload[key] = value
	}
	return payload
}
