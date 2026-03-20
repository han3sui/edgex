package core

import (
	"sync"
)

// RTTManager RTT管理器
type RTTManager struct {
	rttData      map[string][]int64
	ewmaData     map[string]int64
	sampleWindow int
	alpha        float64
	mu           sync.RWMutex
}

// NewRTTManager 创建RTT管理器
func NewRTTManager() *RTTManager {
	return &RTTManager{
		rttData:      make(map[string][]int64),
		ewmaData:     make(map[string]int64),
		sampleWindow: 20,
		alpha:        0.125,
	}
}

// UpdateRTT 更新RTT数据
func (r *RTTManager) UpdateRTT(deviceID string, rtt int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.rttData[deviceID]; !exists {
		r.rttData[deviceID] = make([]int64, 0, r.sampleWindow)
		r.ewmaData[deviceID] = rtt
	}

	r.rttData[deviceID] = append(r.rttData[deviceID], rtt)
	if len(r.rttData[deviceID]) > r.sampleWindow {
		r.rttData[deviceID] = r.rttData[deviceID][1:]
	}

	// 计算EWMA
	currentEWMA := r.ewmaData[deviceID]
	newEWMA := int64(float64(currentEWMA)*(1-r.alpha) + float64(rtt)*r.alpha)
	r.ewmaData[deviceID] = newEWMA
}

// GetEWMARTT 获取EWMA RTT
func (r *RTTManager) GetEWMARTT(deviceID string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if ewma, exists := r.ewmaData[deviceID]; exists {
		return ewma
	}
	return 0
}

// GetRTTSamples 获取RTT采样数据
func (r *RTTManager) GetRTTSamples(deviceID string) []int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if samples, exists := r.rttData[deviceID]; exists {
		// 返回副本，避免外部修改
		samplesCopy := make([]int64, len(samples))
		copy(samplesCopy, samples)
		return samplesCopy
	}
	return nil
}

// GetAverageRTT 获取平均RTT
func (r *RTTManager) GetAverageRTT(deviceID string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	samples, exists := r.rttData[deviceID]
	if !exists || len(samples) == 0 {
		return 0
	}

	total := int64(0)
	for _, rtt := range samples {
		total += rtt
	}

	return total / int64(len(samples))
}

// GetMaxRTT 获取最大RTT
func (r *RTTManager) GetMaxRTT(deviceID string) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	samples, exists := r.rttData[deviceID]
	if !exists || len(samples) == 0 {
		return 0
	}

	max := int64(0)
	for _, rtt := range samples {
		if rtt > max {
			max = rtt
		}
	}

	return max
}

// ClearRTTData 清除设备的RTT数据
func (r *RTTManager) ClearRTTData(deviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.rttData, deviceID)
	delete(r.ewmaData, deviceID)
}
