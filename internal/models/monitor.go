// Package models contains typed structs that mirror Uptime Kuma v2.x
// data shapes as emitted and accepted by the Socket.IO server.
package models

// MonitorType enumerates the supported monitor types in v2.3.x.
type MonitorType string

const (
	MonitorTypeHTTP            MonitorType = "http"
	MonitorTypePort            MonitorType = "port"
	MonitorTypePing            MonitorType = "ping"
	MonitorTypeKeyword         MonitorType = "keyword"
	MonitorTypeGRPCKeyword     MonitorType = "grpc-keyword"
	MonitorTypeDNS             MonitorType = "dns"
	MonitorTypeDocker          MonitorType = "docker"
	MonitorTypeRealBrowser     MonitorType = "real-browser"
	MonitorTypePush            MonitorType = "push"
	MonitorTypeSteam           MonitorType = "steam"
	MonitorTypeGameDig         MonitorType = "gamedig"
	MonitorTypeMQTT            MonitorType = "mqtt"
	MonitorTypeSQLServer       MonitorType = "sqlserver"
	MonitorTypeMySQL           MonitorType = "mysql"
	MonitorTypePostgres        MonitorType = "postgres"
	MonitorTypeMongoDB         MonitorType = "mongodb"
	MonitorTypeRadius          MonitorType = "radius"
	MonitorTypeRedis           MonitorType = "redis"
	MonitorTypeGroup           MonitorType = "group"
	MonitorTypeTailscalePing   MonitorType = "tailscale-ping"
	MonitorTypeWebSocket       MonitorType = "websocket-upgrade"
	MonitorTypeSNMP            MonitorType = "snmp"
	MonitorTypeRabbitMQ        MonitorType = "rabbitmq"
	MonitorTypeSMTP            MonitorType = "smtp"
	MonitorTypeSIPOptions      MonitorType = "sip-options"
	MonitorTypeManual          MonitorType = "manual"
	MonitorTypeGlobalping      MonitorType = "globalping"
	MonitorTypeSystemService   MonitorType = "system-service"
	MonitorTypeOracleDB        MonitorType = "oracledb"
)

// MonitorStatus mirrors Uptime Kuma heartbeat status codes.
type MonitorStatus int

const (
	StatusDown        MonitorStatus = 0
	StatusUp          MonitorStatus = 1
	StatusPending     MonitorStatus = 2
	StatusMaintenance MonitorStatus = 3
)

// Monitor represents an Uptime Kuma monitor as returned by monitorList
// and updateMonitorIntoList events.
//
// Fields use omitempty so that AddMonitorRequest can reuse this struct
// for create/edit operations without sending zero-value fields.
type Monitor struct {
	// Core identity
	ID     int         `json:"id,omitempty"`
	Name   string      `json:"name"`
	Type   MonitorType `json:"type"`
	Active FlexInt `json:"active,omitempty"` // 0 = paused, 1 = active; server sends int or bool

	// Target
	URL          string `json:"url,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	Port         int    `json:"port,omitempty"`
	Method       string `json:"method,omitempty"` // GET, POST, etc.

	// Timing
	Interval      int `json:"interval,omitempty"`      // seconds
	RetryInterval int `json:"retryInterval,omitempty"` // seconds
	MaxRetries    int `json:"maxretries,omitempty"`
	Timeout       int `json:"timeout,omitempty"`       // seconds

	// HTTP options
	Body              string            `json:"body,omitempty"`
	Headers           string            `json:"headers,omitempty"`
	BasicAuthUser     string            `json:"basic_auth_user,omitempty"`
	BasicAuthPass     string            `json:"basic_auth_pass,omitempty"`
	HTTPBodyEncoding  string            `json:"httpBodyEncoding,omitempty"`
	AcceptedStatCodes []string          `json:"accepted_statuscodes,omitempty"`
	IgnoreTLS         bool              `json:"ignoreTls,omitempty"`
	MaxRedirects      int               `json:"maxredirects,omitempty"`
	ExpiryNotify      bool              `json:"expiryNotification,omitempty"`

	// Keyword / JSON path
	Keyword          string `json:"keyword,omitempty"`
	InvertKeyword    bool   `json:"invertKeyword,omitempty"`
	JSONPath         string `json:"jsonPath,omitempty"`
	ExpectedValue    string `json:"expectedValue,omitempty"`
	JSONPathOperator string `json:"jsonPathOperator,omitempty"`

	// DNS
	DNSResolveType   string `json:"dns_resolve_type,omitempty"`
	DNSResolveServer string `json:"dns_resolve_server,omitempty"`
	DNSLastResult    string `json:"dns_last_result,omitempty"`

	// Push monitor
	PushToken string `json:"pushToken,omitempty"`

	// Notifications (map of notificationID → bool)
	NotificationIDList map[string]bool `json:"notificationIDList,omitempty"`

	// Tags (populated in monitorList)
	Tags []MonitorTag `json:"tags,omitempty"`

	// Grouping
	ParentID *int `json:"parent,omitempty"`
	Weight   int  `json:"weight,omitempty"`

	// Proxy
	ProxyID *int `json:"proxyId,omitempty"`

	// TLS / Certificate
	TLSInfo map[string]any `json:"tlsInfo,omitempty"`

	// Uptime stats (read-only, present in monitorList push)
	Uptime24h float64 `json:"uptime,omitempty"`
	Uptime1m  float64 `json:"uptime1m,omitempty"`

	// Conditions (v2 feature for DNS, MQTT, SQL monitors)
	Conditions []MonitorCondition `json:"conditions,omitempty"`
}

// MonitorTag is a tag attached to a monitor.
type MonitorTag struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Value string `json:"value,omitempty"`
}

// MonitorCondition defines a condition for condition-based monitors (DNS, MQTT, etc.).
type MonitorCondition struct {
	ID       string `json:"id"`
	Value    string `json:"value"`
	Operator string `json:"operator"`
}

// AddMonitorRequest is the payload for the "addMonitor" Socket.IO event.
// Only set the fields relevant to the monitor type.
type AddMonitorRequest struct {
	Type              MonitorType     `json:"type"`
	Name              string          `json:"name"`
	URL               string          `json:"url,omitempty"`
	Hostname          string          `json:"hostname,omitempty"`
	Port              int             `json:"port,omitempty"`
	Method            string          `json:"method,omitempty"`
	Interval          int             `json:"interval,omitempty"`
	RetryInterval     int             `json:"retryInterval,omitempty"`
	MaxRetries        int             `json:"maxretries,omitempty"`
	Timeout           int             `json:"timeout,omitempty"`
	Active            bool            `json:"active"`
	Body              string          `json:"body,omitempty"`
	Headers           string          `json:"headers,omitempty"`
	BasicAuthUser     string          `json:"basic_auth_user,omitempty"`
	BasicAuthPass     string          `json:"basic_auth_pass,omitempty"`
	IgnoreTLS         bool            `json:"ignoreTls,omitempty"`
	Keyword           string          `json:"keyword,omitempty"`
	InvertKeyword     bool            `json:"invertKeyword,omitempty"`
	DNSResolveType    string          `json:"dns_resolve_type,omitempty"`
	DNSResolveServer  string          `json:"dns_resolve_server,omitempty"`
	ParentID          *int            `json:"parent,omitempty"`
	ProxyID           *int            `json:"proxyId,omitempty"`
	NotificationIDList map[string]bool `json:"notificationIDList"`
	// accepted_statuscodes: required for HTTP monitors, e.g. ["200-299"]
	AcceptedStatCodes []string           `json:"accepted_statuscodes,omitempty"`
	// conditions: NOT NULL in DB — send [] for monitors that don't use conditions
	Conditions        []MonitorCondition `json:"conditions"`
}

// EditMonitorRequest is the payload for the "editMonitor" event.
// Must include the monitor ID.
type EditMonitorRequest struct {
	AddMonitorRequest
	ID int `json:"id"`
}

// addMonitorResponse is the ACK payload from "addMonitor".
type AddMonitorResponse struct {
	Ok        bool   `json:"ok"`
	Msg       string `json:"msg"`
	MonitorID int    `json:"monitorID"`
}

// MonitorList is the shape of the "monitorList" push event payload.
// Keys are monitor IDs as strings.
type MonitorList map[string]Monitor
