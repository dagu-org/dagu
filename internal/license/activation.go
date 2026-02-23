package license

// ActivationData holds the persisted activation state.
type ActivationData struct {
	Token           string `json:"token"`
	HeartbeatSecret string `json:"heartbeat_secret"`
	LicenseKey      string `json:"license_key"`
	ServerID        string `json:"server_id"`
}

// ActivationStore provides persistence for activation data.
type ActivationStore interface {
	Load() (*ActivationData, error)
	Save(data *ActivationData) error
	Remove() error
}
