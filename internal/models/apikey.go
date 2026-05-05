package models

// APIKey is the public representation emitted by apiKeyList.
type APIKey struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	CreatedAt string  `json:"createdDate"`
	Active    FlexInt `json:"active"`
	Expires   *string `json:"expires"`
}

// AddAPIKeyRequest is the payload for addAPIKey.
type AddAPIKeyRequest struct {
	Name    string  `json:"name"`
	Expires *string `json:"expires"`
}

// AddAPIKeyResponse includes the actual clear-text token returned once.
type AddAPIKeyResponse struct {
	Ok    bool   `json:"ok"`
	Msg   string `json:"msg"`
	Key   string `json:"key"`
	KeyID int    `json:"keyID"`
}
