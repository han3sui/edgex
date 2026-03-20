package core

import (
	"sync"
)

// ProtocolAdapter 协议适配器接口
type ProtocolAdapter interface {
	GetProtocolType() string
	AdjustParameters(deviceID string, params map[string]interface{}) map[string]interface{}
	ValidateParameters(params map[string]interface{}) error
	GetDefaultParameters() map[string]interface{}
}

// ProtocolAdapterRegistry 协议适配器注册表
type ProtocolAdapterRegistry struct {
	adapters map[string]ProtocolAdapter
	mu       sync.RWMutex
}

// NewProtocolAdapterRegistry 创建协议适配器注册表
func NewProtocolAdapterRegistry() *ProtocolAdapterRegistry {
	registry := &ProtocolAdapterRegistry{
		adapters: make(map[string]ProtocolAdapter),
	}

	// 注册默认的协议适配器
	registry.RegisterAdapter(NewModbusProtocolAdapter())
	registry.RegisterAdapter(NewTCPProtocolAdapter())
	registry.RegisterAdapter(NewBACnetProtocolAdapter())

	return registry
}

// RegisterAdapter 注册协议适配器
func (r *ProtocolAdapterRegistry) RegisterAdapter(adapter ProtocolAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.adapters[adapter.GetProtocolType()] = adapter
}

// GetAdapter 获取协议适配器
func (r *ProtocolAdapterRegistry) GetAdapter(protocolType string) ProtocolAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if adapter, exists := r.adapters[protocolType]; exists {
		return adapter
	}

	// 返回默认的协议适配器
	return NewDefaultProtocolAdapter()
}

// DefaultProtocolAdapter 默认协议适配器
type DefaultProtocolAdapter struct{}

// NewDefaultProtocolAdapter 创建默认协议适配器
func NewDefaultProtocolAdapter() *DefaultProtocolAdapter {
	return &DefaultProtocolAdapter{}
}

// GetProtocolType 获取协议类型
func (a *DefaultProtocolAdapter) GetProtocolType() string {
	return "default"
}

// AdjustParameters 调整参数
func (a *DefaultProtocolAdapter) AdjustParameters(deviceID string, params map[string]interface{}) map[string]interface{} {
	// 默认不做调整
	return params
}

// ValidateParameters 验证参数
func (a *DefaultProtocolAdapter) ValidateParameters(params map[string]interface{}) error {
	// 默认验证通过
	return nil
}

// GetDefaultParameters 获取默认参数
func (a *DefaultProtocolAdapter) GetDefaultParameters() map[string]interface{} {
	return map[string]interface{}{
		"timeout":  5000,
		"retry":    3,
		"interval": 10000,
	}
}

// ModbusProtocolAdapter Modbus协议适配器
type ModbusProtocolAdapter struct{}

// NewModbusProtocolAdapter 创建Modbus协议适配器
func NewModbusProtocolAdapter() *ModbusProtocolAdapter {
	return &ModbusProtocolAdapter{}
}

// GetProtocolType 获取协议类型
func (a *ModbusProtocolAdapter) GetProtocolType() string {
	return "modbus"
}

// AdjustParameters 调整参数
func (a *ModbusProtocolAdapter) AdjustParameters(deviceID string, params map[string]interface{}) map[string]interface{} {
	// 根据设备特性调整Modbus参数
	if _, exists := params["batch_size"]; !exists {
		params["batch_size"] = 100
	}

	if _, exists := params["timeout"]; !exists {
		params["timeout"] = 3000
	}

	return params
}

// ValidateParameters 验证参数
func (a *ModbusProtocolAdapter) ValidateParameters(params map[string]interface{}) error {
	// 验证Modbus特定参数
	return nil
}

// GetDefaultParameters 获取默认参数
func (a *ModbusProtocolAdapter) GetDefaultParameters() map[string]interface{} {
	return map[string]interface{}{
		"batch_size": 100,
		"timeout":    3000,
		"retry":      3,
		"interval":   10000,
	}
}

// TCPProtocolAdapter TCP协议适配器
type TCPProtocolAdapter struct{}

// NewTCPProtocolAdapter 创建TCP协议适配器
func NewTCPProtocolAdapter() *TCPProtocolAdapter {
	return &TCPProtocolAdapter{}
}

// GetProtocolType 获取协议类型
func (a *TCPProtocolAdapter) GetProtocolType() string {
	return "tcp"
}

// AdjustParameters 调整参数
func (a *TCPProtocolAdapter) AdjustParameters(deviceID string, params map[string]interface{}) map[string]interface{} {
	// 根据设备特性调整TCP参数
	if _, exists := params["buffer_size"]; !exists {
		params["buffer_size"] = 4096
	}

	if _, exists := params["timeout"]; !exists {
		params["timeout"] = 5000
	}

	return params
}

// ValidateParameters 验证参数
func (a *TCPProtocolAdapter) ValidateParameters(params map[string]interface{}) error {
	// 验证TCP特定参数
	return nil
}

// GetDefaultParameters 获取默认参数
func (a *TCPProtocolAdapter) GetDefaultParameters() map[string]interface{} {
	return map[string]interface{}{
		"buffer_size": 4096,
		"timeout":     5000,
		"retry":       3,
		"interval":    10000,
	}
}

// BACnetProtocolAdapter BACnet协议适配器
type BACnetProtocolAdapter struct{}

// NewBACnetProtocolAdapter 创建BACnet协议适配器
func NewBACnetProtocolAdapter() *BACnetProtocolAdapter {
	return &BACnetProtocolAdapter{}
}

// GetProtocolType 获取协议类型
func (a *BACnetProtocolAdapter) GetProtocolType() string {
	return "bacnet"
}

// AdjustParameters 调整参数
func (a *BACnetProtocolAdapter) AdjustParameters(deviceID string, params map[string]interface{}) map[string]interface{} {
	// 根据设备特性调整BACnet参数
	if _, exists := params["apdu_timeout"]; !exists {
		params["apdu_timeout"] = 2000
	}

	if _, exists := params["max_apdu"]; !exists {
		params["max_apdu"] = 1476
	}

	return params
}

// ValidateParameters 验证参数
func (a *BACnetProtocolAdapter) ValidateParameters(params map[string]interface{}) error {
	// 验证BACnet特定参数
	return nil
}

// GetDefaultParameters 获取默认参数
func (a *BACnetProtocolAdapter) GetDefaultParameters() map[string]interface{} {
	return map[string]interface{}{
		"apdu_timeout": 2000,
		"max_apdu":     1476,
		"retry":        3,
		"interval":     10000,
	}
}
