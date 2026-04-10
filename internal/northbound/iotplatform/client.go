package iotplatform

import (
	"edge-gateway/internal/model"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	StatusDisconnected = 0
	StatusConnected    = 1
	StatusReconnecting = 2
	StatusError        = 3

	component = "iot-platform"
)

// ChannelManager is the subset of core.ChannelManager used by this package.
type ChannelManager interface {
	AddChannel(ch *model.Channel) error
	RemoveChannel(channelID string) error
	StartChannel(channelID string) error
	StopChannel(channelID string) error
	GetChannel(channelID string) *model.Channel
	WritePoint(channelID, deviceID, pointID string, value any) error
	GetChannels() []model.Channel
}

type ClientStats struct {
	SuccessCount    int64 `json:"success_count"`
	FailCount       int64 `json:"fail_count"`
	ReconnectCount  int64 `json:"reconnect_count"`
	LastOnlineTime  int64 `json:"last_online_time"`
	LastOfflineTime int64 `json:"last_offline_time"`
}

type Client struct {
	config   model.IotPlatformConfig
	configMu sync.RWMutex

	client   mqtt.Client
	status   int
	statusMu sync.RWMutex
	stopChan chan struct{}

	cm ChannelManager

	// Aggregation buffer: deviceID -> pointID -> Value
	bufferMu    sync.Mutex
	buffers     map[string]map[string]model.Value
	bufferTimer *time.Timer

	successCount    int64
	failCount       int64
	reconnectCount  int64
	lastOnlineTime  int64
	lastOfflineTime int64
}

func NewClient(cfg model.IotPlatformConfig, cm ChannelManager) *Client {
	return &Client{
		config:   cfg,
		cm:       cm,
		stopChan: make(chan struct{}),
		buffers:  make(map[string]map[string]model.Value),
	}
}

func (c *Client) GetStatus() int {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status
}

func (c *Client) GetStats() ClientStats {
	return ClientStats{
		SuccessCount:    atomic.LoadInt64(&c.successCount),
		FailCount:       atomic.LoadInt64(&c.failCount),
		ReconnectCount:  atomic.LoadInt64(&c.reconnectCount),
		LastOnlineTime:  atomic.LoadInt64(&c.lastOnlineTime),
		LastOfflineTime: atomic.LoadInt64(&c.lastOfflineTime),
	}
}

func (c *Client) setStatus(s int) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status = s
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func (c *Client) Start() error {
	go c.connectLoop()
	return nil
}

func (c *Client) Stop() {
	select {
	case <-c.stopChan:
		return
	default:
		close(c.stopChan)
	}
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(2000)
	}
	c.setStatus(StatusDisconnected)
}

func (c *Client) UpdateConfig(cfg model.IotPlatformConfig) error {
	c.configMu.RLock()
	needRestart := c.config.Broker != cfg.Broker ||
		c.config.ClientID != cfg.ClientID ||
		c.config.Username != cfg.Username ||
		c.config.GatewayID != cfg.GatewayID ||
		c.config.ProductID != cfg.ProductID ||
		c.config.Password != cfg.Password
	c.configMu.RUnlock()

	c.configMu.Lock()
	c.config = cfg
	c.configMu.Unlock()

	if needRestart {
		c.Stop()
		c.stopChan = make(chan struct{})
		return c.Start()
	}
	return nil
}

// ---------------------------------------------------------------------------
// MQTT connection
// ---------------------------------------------------------------------------

func (c *Client) connectLoop() {
	c.configMu.RLock()
	broker := c.config.Broker
	clientID := c.config.ClientID
	username := c.config.Username
	password := c.config.Password
	productID := c.config.ProductID
	gatewayID := c.config.GatewayID
	c.configMu.RUnlock()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(false)

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		zap.L().Info("IoT platform MQTT connected",
			zap.String("broker", broker),
			zap.String("component", component),
		)
		c.setStatus(StatusConnected)
		atomic.StoreInt64(&c.lastOnlineTime, time.Now().UnixMilli())
		c.subscribe(client, productID, gatewayID)
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		zap.L().Warn("IoT platform MQTT connection lost",
			zap.Error(err),
			zap.String("component", component),
		)
		c.setStatus(StatusDisconnected)
		atomic.StoreInt64(&c.lastOfflineTime, time.Now().UnixMilli())
		go c.reconnectLoop()
	})

	c.client = mqtt.NewClient(opts)

	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		zap.L().Error("IoT platform initial MQTT connection failed",
			zap.Error(token.Error()),
			zap.String("component", component),
		)
		c.setStatus(StatusDisconnected)
		atomic.StoreInt64(&c.lastOfflineTime, time.Now().UnixMilli())
		go c.reconnectLoop()
	}
}

func (c *Client) reconnectLoop() {
	retryCount := 0
	for {
		select {
		case <-c.stopChan:
			return
		default:
		}
		c.setStatus(StatusReconnecting)
		token := c.client.Connect()
		if token.Wait() && token.Error() == nil {
			atomic.AddInt64(&c.reconnectCount, 1)
			return
		}
		retryCount++
		if retryCount <= 10 {
			time.Sleep(3 * time.Second)
		} else {
			c.setStatus(StatusError)
			time.Sleep(60 * time.Second)
			retryCount = 0
		}
	}
}

// ---------------------------------------------------------------------------
// Topic helpers
// ---------------------------------------------------------------------------

func (c *Client) topicPrefix(productID, entityID string) string {
	return fmt.Sprintf("/sys/%s/%s", productID, entityID)
}

func (c *Client) subscribe(client mqtt.Client, productID, gatewayID string) {
	prefix := c.topicPrefix(productID, gatewayID)

	// Config push (includes config.push and config.delete)
	configTopic := prefix + "/thing/config/push"
	if token := client.Subscribe(configTopic, 1, c.handleConfigPush); token.Wait() && token.Error() != nil {
		zap.L().Error("Failed to subscribe config/push", zap.Error(token.Error()), zap.String("component", component))
	} else {
		zap.L().Info("Subscribed", zap.String("topic", configTopic), zap.String("component", component))
	}

	// Property set (wildcard for all devices under this gateway)
	propSetTopic := fmt.Sprintf("/sys/%s/+/thing/property/set", productID)
	if token := client.Subscribe(propSetTopic, 1, c.handlePropertySet); token.Wait() && token.Error() != nil {
		zap.L().Error("Failed to subscribe property/set", zap.Error(token.Error()), zap.String("component", component))
	} else {
		zap.L().Info("Subscribed", zap.String("topic", propSetTopic), zap.String("component", component))
	}

	// Service invoke (wildcard)
	svcTopic := fmt.Sprintf("/sys/%s/+/thing/service/+/invoke", productID)
	if token := client.Subscribe(svcTopic, 1, c.handleServiceInvoke); token.Wait() && token.Error() != nil {
		zap.L().Error("Failed to subscribe service/invoke", zap.Error(token.Error()), zap.String("component", component))
	} else {
		zap.L().Info("Subscribed", zap.String("topic", svcTopic), zap.String("component", component))
	}
}

// ---------------------------------------------------------------------------
// Config push / delete handler
// ---------------------------------------------------------------------------

func (c *Client) handleConfigPush(_ mqtt.Client, msg mqtt.Message) {
	zap.L().Info("Received config push", zap.String("topic", msg.Topic()), zap.String("component", component))

	var raw struct {
		ID        string          `json:"id"`
		Version   string          `json:"version"`
		Timestamp int64           `json:"timestamp"`
		Method    string          `json:"method"`
		Params    json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(msg.Payload(), &raw); err != nil {
		zap.L().Error("Failed to parse config push envelope", zap.Error(err), zap.String("component", component))
		return
	}

	switch raw.Method {
	case "thing.config.push":
		c.processConfigPush(raw.ID, raw.Params)
	case "thing.config.delete":
		c.processConfigDelete(raw.ID, raw.Params)
	default:
		zap.L().Warn("Unknown config method", zap.String("method", raw.Method), zap.String("component", component))
	}
}

func (c *Client) processConfigPush(msgID string, paramsRaw json.RawMessage) {
	var params ConfigPushParams
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		zap.L().Error("Failed to parse config push params", zap.Error(err), zap.String("component", component))
		c.replyConfig(msgID, 400, false, "invalid params: "+err.Error())
		return
	}

	ch, err := BuildChannel(&params)
	if err != nil {
		zap.L().Error("Failed to build channel from platform config", zap.Error(err), zap.String("component", component))
		c.replyConfig(msgID, 500, false, err.Error())
		return
	}

	// Remove existing channel with same ID (full replace semantics)
	if existing := c.cm.GetChannel(ch.ID); existing != nil {
		_ = c.cm.StopChannel(ch.ID)
		_ = c.cm.RemoveChannel(ch.ID)
	}

	if err := c.cm.AddChannel(ch); err != nil {
		zap.L().Error("Failed to add channel", zap.Error(err), zap.String("component", component))
		c.replyConfig(msgID, 500, false, err.Error())
		return
	}

	c.configMu.RLock()
	autoStart := c.config.AutoStart
	c.configMu.RUnlock()

	if autoStart || ch.Enable {
		if err := c.cm.StartChannel(ch.ID); err != nil {
			zap.L().Error("Channel added but failed to start", zap.Error(err), zap.String("component", component))
			c.replyConfig(msgID, 500, false, "channel added but start failed: "+err.Error())
			return
		}
	}

	zap.L().Info("Platform config applied",
		zap.String("channel", ch.ID),
		zap.Int("devices", len(ch.Devices)),
		zap.String("component", component),
	)
	c.replyConfig(msgID, 200, true, "ok")
}

func (c *Client) processConfigDelete(msgID string, paramsRaw json.RawMessage) {
	var params ConfigDeleteParams
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		c.replyConfig(msgID, 400, false, "invalid params: "+err.Error())
		return
	}

	channelID := MakeChannelID(params.ChannelID)
	if existing := c.cm.GetChannel(channelID); existing == nil {
		c.replyConfig(msgID, 200, true, "ok")
		return
	}

	_ = c.cm.StopChannel(channelID)
	if err := c.cm.RemoveChannel(channelID); err != nil {
		c.replyConfig(msgID, 500, false, err.Error())
		return
	}

	zap.L().Info("Platform config deleted", zap.String("channel", channelID), zap.String("component", component))
	c.replyConfig(msgID, 200, true, "ok")
}

func (c *Client) replyConfig(msgID string, code int, success bool, message string) {
	reply := ConfigReply{
		ID:      msgID,
		Code:    code,
		Success: success,
		Message: message,
	}
	data, _ := json.Marshal(reply)

	c.configMu.RLock()
	productID := c.config.ProductID
	gatewayID := c.config.GatewayID
	c.configMu.RUnlock()

	topic := c.topicPrefix(productID, gatewayID) + "/thing/config/reply"
	c.publish(topic, 1, data)
}

// ---------------------------------------------------------------------------
// Property set handler
// ---------------------------------------------------------------------------

func (c *Client) handlePropertySet(_ mqtt.Client, msg mqtt.Message) {
	// Topic: /sys/{productID}/{deviceID}/thing/property/set
	deviceID := c.extractDeviceIDFromTopic(msg.Topic(), "property/set")
	if deviceID == "" {
		return
	}

	var req PropertySetMessage
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		zap.L().Error("Failed to parse property/set", zap.Error(err), zap.String("component", component))
		return
	}

	zap.L().Info("Received property set",
		zap.String("device", deviceID),
		zap.Any("data", req.Data),
		zap.String("component", component),
	)

	channelID := c.findChannelForDevice(deviceID)
	if channelID == "" {
		c.replyPropertySet(req.ID, deviceID, 404, false, "device not found in any channel")
		return
	}

	var errs []string
	for modelCode, val := range req.Data {
		if err := c.cm.WritePoint(channelID, deviceID, modelCode, val); err != nil {
			errs = append(errs, modelCode+": "+err.Error())
		}
	}

	if len(errs) > 0 {
		c.replyPropertySet(req.ID, deviceID, 500, false, strings.Join(errs, "; "))
	} else {
		c.replyPropertySet(req.ID, deviceID, 200, true, "ok")
	}
}

func (c *Client) replyPropertySet(msgID, deviceID string, code int, success bool, message string) {
	reply := PropertySetReply{
		ID:      msgID,
		Code:    code,
		Success: success,
		Message: message,
	}
	data, _ := json.Marshal(reply)

	c.configMu.RLock()
	productID := c.config.ProductID
	c.configMu.RUnlock()

	topic := c.topicPrefix(productID, deviceID) + "/thing/property/set_reply"
	c.publish(topic, 1, data)
}

// ---------------------------------------------------------------------------
// Service invoke handler
// ---------------------------------------------------------------------------

func (c *Client) handleServiceInvoke(_ mqtt.Client, msg mqtt.Message) {
	// Topic: /sys/{productID}/{deviceID}/thing/service/{serviceCode}/invoke
	parts := strings.Split(msg.Topic(), "/")
	// Expected: ["", "sys", productID, deviceID, "thing", "service", serviceCode, "invoke"]
	if len(parts) < 8 {
		return
	}
	deviceID := parts[3]
	serviceCode := parts[6]

	var req ServiceInvokeMessage
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		zap.L().Error("Failed to parse service/invoke", zap.Error(err), zap.String("component", component))
		return
	}

	zap.L().Info("Received service invoke",
		zap.String("device", deviceID),
		zap.String("service", serviceCode),
		zap.Any("data", req.Data),
		zap.String("component", component),
	)

	// Currently no built-in service execution; reply with not-implemented
	reply := ServiceInvokeReply{
		ID:      req.ID,
		Code:    501,
		Success: false,
		Message: fmt.Sprintf("service %s not implemented", serviceCode),
	}
	data, _ := json.Marshal(reply)

	c.configMu.RLock()
	productID := c.config.ProductID
	c.configMu.RUnlock()

	replyTopic := c.topicPrefix(productID, deviceID) + fmt.Sprintf("/thing/service/%s/invoke_reply", serviceCode)
	c.publish(replyTopic, 1, data)
}

// ---------------------------------------------------------------------------
// Data reporting (called by NorthboundManager via pipeline)
// ---------------------------------------------------------------------------

func (c *Client) Publish(v model.Value) {
	if c.client == nil || !c.client.IsConnected() {
		return
	}

	// Only report data from channels managed by this platform
	if !IsPlatformChannel(v.ChannelID) {
		return
	}

	// Skip bad-quality values
	if v.Quality != "" && v.Quality != "Good" {
		return
	}

	c.bufferMu.Lock()
	defer c.bufferMu.Unlock()

	if c.buffers[v.DeviceID] == nil {
		c.buffers[v.DeviceID] = make(map[string]model.Value)
	}
	c.buffers[v.DeviceID][v.PointID] = v

	// Debounce: flush after 200ms to aggregate multiple points
	if c.bufferTimer != nil {
		c.bufferTimer.Stop()
	}
	c.bufferTimer = time.AfterFunc(200*time.Millisecond, c.flushBuffers)
}

func (c *Client) flushBuffers() {
	c.bufferMu.Lock()
	if len(c.buffers) == 0 {
		c.bufferMu.Unlock()
		return
	}

	// Build data map: deviceID -> { modelCode -> value }
	dataMap := make(map[string]any, len(c.buffers))
	for devID, points := range c.buffers {
		vals := make(map[string]any, len(points))
		for pointID, v := range points {
			vals[pointID] = v.Value
		}
		dataMap[devID] = vals
	}

	// Clear buffers
	c.buffers = make(map[string]map[string]model.Value)
	c.bufferMu.Unlock()

	msg := GatewayPostMessage{
		ID:   uuid.New().String(),
		Sys:  GatewayPostSys{Ack: false},
		Time: time.Now().UnixMilli(),
		Data: dataMap,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		zap.L().Error("Failed to marshal gateway/post", zap.Error(err), zap.String("component", component))
		return
	}

	c.configMu.RLock()
	productID := c.config.ProductID
	gatewayID := c.config.GatewayID
	c.configMu.RUnlock()

	topic := c.topicPrefix(productID, gatewayID) + "/thing/gateway/post"
	c.publish(topic, 0, payload)
}

// ---------------------------------------------------------------------------
// System metrics reporting (called by SysMonitor subscriber)
// ---------------------------------------------------------------------------

// PublishSystemMetrics publishes gateway self attributes via property/post.
// Topic: /sys/{productID}/{gatewayID}/thing/property/post
func (c *Client) PublishSystemMetrics(metrics map[string]any) {
	if c.client == nil || !c.client.IsConnected() {
		return
	}

	msg := PropertyPostMessage{
		ID:   uuid.New().String(),
		Sys:  GatewayPostSys{Ack: false},
		Time: time.Now().UnixMilli(),
		Data: metrics,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		zap.L().Error("Failed to marshal system metrics", zap.Error(err), zap.String("component", component))
		return
	}

	c.configMu.RLock()
	productID := c.config.ProductID
	gatewayID := c.config.GatewayID
	c.configMu.RUnlock()

	topic := c.topicPrefix(productID, gatewayID) + "/thing/property/post"
	c.publish(topic, 0, payload)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (c *Client) publish(topic string, qos byte, payload []byte) {
	if c.client == nil || !c.client.IsConnected() {
		return
	}
	token := c.client.Publish(topic, qos, false, payload)
	go func() {
		if token.Wait() && token.Error() != nil {
			atomic.AddInt64(&c.failCount, 1)
			zap.L().Error("IoT platform publish failed",
				zap.String("topic", topic),
				zap.Error(token.Error()),
				zap.String("component", component),
			)
		} else {
			atomic.AddInt64(&c.successCount, 1)
			zap.L().Debug("IoT platform published",
				zap.String("topic", topic),
				zap.Int("bytes", len(payload)),
				zap.String("component", component),
			)
		}
	}()
}

// extractDeviceIDFromTopic extracts deviceID from topics like /sys/{productID}/{deviceID}/thing/...
func (c *Client) extractDeviceIDFromTopic(topic, suffix string) string {
	parts := strings.Split(topic, "/")
	// ["", "sys", productID, deviceID, "thing", ...]
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

// findChannelForDevice searches all platform-managed channels for a device.
func (c *Client) findChannelForDevice(deviceID string) string {
	for _, ch := range c.cm.GetChannels() {
		if !IsPlatformChannel(ch.ID) {
			continue
		}
		for _, dev := range ch.Devices {
			if dev.ID == deviceID {
				return ch.ID
			}
		}
	}
	return ""
}
