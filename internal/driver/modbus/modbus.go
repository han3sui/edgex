package modbus

import (
	"context"
	"edge-gateway/internal/driver"
	"edge-gateway/internal/model"
	"log"
	"time"
)

func init() {
	driver.RegisterDriver("modbus-tcp", NewModbusDriver)
	driver.RegisterDriver("modbus-rtu", NewModbusDriver)
	driver.RegisterDriver("modbus-rtu-over-tcp", NewModbusDriver)
}

// ModbusDriver implements driver.Driver interface
type ModbusDriver struct {
	config      model.DriverConfig
	transport   *ModbusTransport
	scheduler   *PointScheduler
	state       *DeviceStateMachine
	probeEngine *ProbeEngine
	addressMap  *ValidAddressMap

	// Kept for direct access if needed, though mostly delegating
	slaveID uint8
}

func NewModbusDriver() driver.Driver {
	return &ModbusDriver{
		state: NewDeviceStateMachine(),
	}
}

func (d *ModbusDriver) Init(config model.DriverConfig) error {
	d.config = config

	// Parse configuration
	d.slaveID = 1
	if v, ok := config.Config["slave_id"]; ok {
		switch val := v.(type) {
		case int:
			d.slaveID = uint8(val)
		case float64:
			d.slaveID = uint8(val)
		}
	}

	startAddress := 0
	if v, ok := config.Config["startAddress"]; ok {
		if f, ok := v.(float64); ok {
			startAddress = int(f)
		} else if i, ok := v.(int); ok {
			startAddress = i
		}
	}

	byteOrder4 := "ABCD"
	if v, ok := config.Config["byteOrder"]; ok {
		byteOrder4 = v.(string)
	}

	batchSize := 50
	if v, ok := config.Config["batchSize"]; ok {
		if f, ok := v.(float64); ok {
			batchSize = int(f)
		} else if i, ok := v.(int); ok {
			batchSize = i
		}
	}

	instructionInterval := 10 * time.Millisecond // 默认 10ms 间隔以提升稳定性
	if v, ok := config.Config["instructionInterval"]; ok {
		if f, ok := v.(float64); ok {
			instructionInterval = time.Duration(f) * time.Millisecond
		} else if i, ok := v.(int); ok {
			instructionInterval = time.Duration(i) * time.Millisecond
		}
	}

	// Initialize components
	d.transport = NewModbusTransport(config)

	// 如果全局指标收集器已初始化，注入到 transport（用于记录请求/点位调试信息）
	if mc := model.GetGlobalMetricsCollector(); mc != nil {
		d.transport.SetMetricsRecorder(mc, config.ChannelID)
	}

	// 设置初始从机 ID
	d.transport.SetUnitID(d.slaveID)

	decoder := NewPointDecoder(byteOrder4, startAddress)

	if v, ok := config.Config["use_dataformat_decoder"]; ok {
		switch val := v.(type) {
		case bool:
			decoder.EnableDataformatDecoder(val)
		case string:
			if val == "true" || val == "1" {
				decoder.EnableDataformatDecoder(true)
			}
		}
	}

	// Max packet size for Modbus TCP/RTU is typically around 250 bytes (120 registers)
	// We use 120 registers (240 bytes) as safe limit
	d.scheduler = NewPointScheduler(d.transport, decoder, 120, uint16(batchSize), instructionInterval)

	// Initialize smart probing engine if enabled
	enableSmartProbe := false
	if v, ok := config.Config["enableSmartProbe"]; ok {
		switch val := v.(type) {
		case bool:
			enableSmartProbe = val
		case string:
			enableSmartProbe = val == "true" || val == "1"
		}
	}

	if enableSmartProbe {
		probeConfig := ProbeConfig{
			MaxDepth:       6,
			Timeout:        3 * time.Second,
			MaxConsecutive: 20,
			EnableMTUProbe: true,
			PersistPath:    "./data/modbus_probe_cache.json",
		}
		if v, ok := config.Config["probeMaxDepth"]; ok {
			if f, ok := v.(float64); ok {
				probeConfig.MaxDepth = int(f)
			}
		}
		if v, ok := config.Config["probeTimeout"]; ok {
			if f, ok := v.(float64); ok {
				probeConfig.Timeout = time.Duration(f) * time.Millisecond
			}
		}
		if v, ok := config.Config["probeMaxConsecutive"]; ok {
			if f, ok := v.(float64); ok {
				probeConfig.MaxConsecutive = int(f)
			}
		}
		if v, ok := config.Config["probeEnableMTU"]; ok {
			switch val := v.(type) {
			case bool:
				probeConfig.EnableMTUProbe = val
			case string:
				probeConfig.EnableMTUProbe = val == "true" || val == "1"
			}
		}

		d.probeEngine = NewProbeEngine(d.transport, probeConfig)
		d.addressMap = NewValidAddressMap(d.probeEngine)
		d.scheduler.SetAddressMap(d.addressMap)

		log.Printf("Modbus smart probing enabled for channel %s", config.ChannelID)
	}

	// Perform a quick MTU probe with timeout to adjust scheduler max packet size
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if mtu, err := d.transport.DetectMTU(ctx); err == nil {
			d.scheduler.SetMaxPacketSize(mtu)
			log.Printf("Modbus MTU probe detected max registers: %d", mtu)
		} else {
			log.Printf("Modbus MTU probe failed: %v", err)
		}
	}()

	return nil
}

func (d *ModbusDriver) Connect(ctx context.Context) error {
	err := d.transport.Connect(ctx)
	if err != nil {
		d.state.OnFailure()
		return err
	}
	d.state.OnSuccess()
	return nil
}

func (d *ModbusDriver) Disconnect() error {
	return d.transport.Disconnect()
}

func (d *ModbusDriver) Health() driver.HealthStatus {
	if d.transport.IsConnected() && d.state.GetState() == StateOnline {
		return driver.HealthStatusGood
	}
	if !d.transport.IsConnected() {
		return driver.HealthStatusBad
	}
	// Maybe degraded? For now return Bad if not online
	return driver.HealthStatusBad
}

func (d *ModbusDriver) ReadPoints(ctx context.Context, points []model.Point) (map[string]model.Value, error) {
	if !d.transport.IsConnected() {
		// Try to reconnect
		if err := d.Connect(ctx); err != nil {
			return nil, err
		}
	}

	// Check if device is in PROBING state
	if d.state.GetState() == StateProbing {
		// Skip state updates during probing to avoid affecting health scoring
		results, err := d.scheduler.Read(ctx, points)
		return results, err
	}

	results, err := d.scheduler.Read(ctx, points)
	if err != nil {
		d.state.OnFailure()
		return results, err
	}
	d.state.OnSuccess()
	return results, nil
}

// WritePoint writes a single point
func (d *ModbusDriver) WritePoint(ctx context.Context, point model.Point, value any) error {
	if !d.transport.IsConnected() {
		if err := d.Connect(ctx); err != nil {
			return err
		}
	}

	err := d.scheduler.Write(ctx, point, value)
	if err != nil {
		d.state.OnFailure()
		return err
	}
	d.state.OnSuccess()
	return nil
}

// SetSlaveID sets the unit ID for subsequent requests
// This is used for devices that support dynamic slave ID switching or when managing multiple slaves over one connection
func (d *ModbusDriver) SetSlaveID(slaveID uint8) error {
	d.slaveID = slaveID
	d.transport.SetUnitID(slaveID)
	log.Printf("ModbusDriver SetSlaveID: changed to %d", slaveID)
	return nil
}

// SetDeviceConfig updates connection parameters dynamically
// This is required by the Driver interface but Modbus might not use it heavily if URL is fixed
func (d *ModbusDriver) SetDeviceConfig(config map[string]any) error {
	// Not implemented for Modbus yet, as config is usually set at Init
	return nil
}

func (d *ModbusDriver) GetConnectionMetrics() (connectionSeconds int64, reconnectCount int64, localAddr string, remoteAddr string, lastDisconnectTime time.Time) {
	if d.transport != nil {
		return d.transport.GetConnectionMetrics()
	}
	return 0, 0, "", "", time.Time{}
}

// ReadPointsWithSlaveID reads points from a specific slave ID
// It sets the slave ID, reads points, and keeps the slave ID set for subsequent calls until changed
func (d *ModbusDriver) ReadPointsWithSlaveID(ctx context.Context, slaveID uint8, points []model.Point) (map[string]model.Value, error) {
	d.SetSlaveID(slaveID)
	return d.ReadPoints(ctx, points)
}

// ProbeDevice performs smart address probing for a specific device (slave ID)
// This is isolated from normal collection and doesn't affect health scoring
func (d *ModbusDriver) ProbeDevice(ctx context.Context, slaveID uint8, regType string, startAddr uint16, endAddr uint16) *DeviceProbeResult {
	if d.probeEngine == nil {
		log.Printf("[Probe] Smart probing not enabled, returning nil")
		return nil
	}

	// Set device state to PROBING to isolate from health scoring
	d.state.SetProbing()
	defer d.state.SetRunning()

	originalSlaveID := d.slaveID
	defer func() {
		d.transport.SetUnitID(originalSlaveID)
	}()

	result := d.probeEngine.ProbeDevice(ctx, slaveID, regType, startAddr, endAddr)
	if result != nil {
		d.scheduler.SetSlaveID(slaveID)
	}
	return result
}

// TriggerReprobe forces a new probe for a specific device
// Useful when device behavior changes or after firmware updates
func (d *ModbusDriver) TriggerReprobe(ctx context.Context, slaveID uint8, regType string, startAddr uint16, endAddr uint16) {
	if d.probeEngine == nil {
		log.Printf("[Probe] Smart probing not enabled, cannot reprobe")
		return
	}
	d.probeEngine.TriggerReprobe(ctx, slaveID, regType, startAddr, endAddr)
}

// GetProbeResult returns cached probe result for a device
func (d *ModbusDriver) GetProbeResult(slaveID uint8, regType string) *DeviceProbeResult {
	if d.probeEngine == nil {
		return nil
	}
	return d.probeEngine.GetCachedResult(slaveID, regType)
}
