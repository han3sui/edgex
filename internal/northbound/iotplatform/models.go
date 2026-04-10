package iotplatform

// PlatformMessage is the generic envelope for all platform MQTT messages.
type PlatformMessage struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	Timestamp int64  `json:"timestamp"`
	Method    string `json:"method"`
	Params    any    `json:"params"`
}

// ConfigPushParams carries the payload of a thing.config.push message.
type ConfigPushParams struct {
	ConfigVersion int64              `json:"config_version"`
	Channel       PlatformChannel    `json:"channel"`
	Devices       []PlatformDevice   `json:"devices"`
}

type PlatformChannel struct {
	ChannelID       int            `json:"channel_id"`
	Name            string         `json:"name"`
	Protocol        string         `json:"protocol"`
	ConnConfig      map[string]any `json:"conn_config"`
	CollectInterval int            `json:"collect_interval"`
}

type PlatformDevice struct {
	DeviceID        string           `json:"device_id"`
	DeviceAddr      map[string]any   `json:"device_addr"`
	CollectInterval int              `json:"collect_interval"`
	Points          []PlatformPoint  `json:"points"`
}

type PlatformPoint struct {
	ModelCode      string  `json:"model_code"`
	FunctionCode   int     `json:"function_code,omitempty"`
	RegisterAddr   int     `json:"register_addr,omitempty"`
	RegisterCount  int     `json:"register_count,omitempty"`
	DataFormat     string  `json:"data_format,omitempty"`
	ByteOrder      string  `json:"byte_order,omitempty"`
	Scale          float64 `json:"scale,omitempty"`
	Offset         float64 `json:"offset,omitempty"`
	DecimalDigits  int     `json:"decimal_digits,omitempty"`
	DataIdentifier string  `json:"data_identifier,omitempty"` // DLT645
	DataLength     int     `json:"data_length,omitempty"`     // DLT645
}

// ConfigDeleteParams carries the payload of a thing.config.delete message.
type ConfigDeleteParams struct {
	ChannelID int `json:"channel_id"`
}

// ConfigReply is sent back to the platform after processing a config push/delete.
type ConfigReply struct {
	ID      string `json:"id"`
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// GatewayPostMessage is the data report message sent from gateway to platform.
type GatewayPostMessage struct {
	ID   string         `json:"id"`
	Sys  GatewayPostSys `json:"sys"`
	Time int64          `json:"time"`
	Data map[string]any `json:"data"` // deviceID -> { modelCode -> value }
}

type GatewayPostSys struct {
	Ack bool `json:"ack"`
}

// PropertyPostMessage is the device self attribute report message.
// Topic: /sys/{productID}/{deviceID}/thing/property/post
type PropertyPostMessage struct {
	ID   string         `json:"id"`
	Sys  GatewayPostSys `json:"sys"`
	Time int64          `json:"time"`
	Data map[string]any `json:"data"` // modelCode -> value
}

// PropertySetMessage is received when the platform sets device properties.
type PropertySetMessage struct {
	ID   string         `json:"id"`
	Time int64          `json:"time"`
	Data map[string]any `json:"data"` // modelCode -> value
}

// PropertySetReply is sent back after processing a property set.
type PropertySetReply struct {
	ID      string `json:"id"`
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ServiceInvokeMessage is received when the platform invokes a device service.
type ServiceInvokeMessage struct {
	ID   string         `json:"id"`
	Time int64          `json:"time"`
	Data map[string]any `json:"data"`
}

// ServiceInvokeReply is sent back after processing a service invocation.
type ServiceInvokeReply struct {
	ID      string         `json:"id"`
	Code    int            `json:"code"`
	Success bool           `json:"success"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}
