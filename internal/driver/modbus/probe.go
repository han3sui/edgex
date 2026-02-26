package modbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type ProbeConfig struct {
	MaxDepth       int
	Timeout        time.Duration
	MaxConsecutive int
	EnableMTUProbe bool
	PersistPath    string
}

type ValidBlock struct {
	Start uint16
	End   uint16
}

type MTUMeasurement struct {
	BatchSize int
	RTT       time.Duration
}

type RTTModel struct {
	Samples map[int][]time.Duration
}

func NewRTTModel() *RTTModel {
	return &RTTModel{
		Samples: make(map[int][]time.Duration),
	}
}

func (m *RTTModel) Record(size int, rtt time.Duration) {
	m.Samples[size] = append(m.Samples[size], rtt)
}

func (m *RTTModel) BestBatchSize() int {
	if len(m.Samples) == 0 {
		return 40
	}

	bestSize := 1
	bestCost := math.MaxFloat64

	for size, samples := range m.Samples {
		if len(samples) == 0 {
			continue
		}
		avg := average(samples)
		cost := float64(avg.Milliseconds()) / float64(size)

		if cost < bestCost {
			bestCost = cost
			bestSize = size
		}
	}

	return bestSize
}

func average(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	total := time.Duration(0)
	for _, d := range durations {
		total += d
	}

	return total / time.Duration(len(durations))
}

type DeviceProbeResult struct {
	SlaveID       uint8
	RegType       string
	Blocks        []ValidBlock
	MaxBatchSize  int
	RTTModel      *RTTModel
	LastProbeTime time.Time
}

type ProbeEngine struct {
	transport Transport
	config    ProbeConfig
	mu        sync.RWMutex
	cache     map[string]*DeviceProbeResult
	probing   map[string]bool
}

func NewProbeEngine(transport Transport, config ProbeConfig) *ProbeEngine {
	if config.MaxDepth == 0 {
		config.MaxDepth = 6
	}
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Second
	}
	if config.MaxConsecutive == 0 {
		config.MaxConsecutive = 20
	}
	if config.PersistPath == "" {
		config.PersistPath = "./data/modbus_probe_cache.json"
	}

	engine := &ProbeEngine{
		transport: transport,
		config:    config,
		cache:     make(map[string]*DeviceProbeResult),
		probing:   make(map[string]bool),
	}

	os.MkdirAll(filepath.Dir(config.PersistPath), 0755)
	engine.loadPersistedCache()

	return engine
}

func (e *ProbeEngine) ProbeDevice(ctx context.Context, slaveID uint8, regType string, startAddr uint16, endAddr uint16) *DeviceProbeResult {
	key := e.cacheKey(slaveID, regType)

	e.mu.RLock()
	if cached, exists := e.cache[key]; exists {
		e.mu.RUnlock()
		log.Printf("[Probe] Using cached probe result for slave %d, type %s", slaveID, regType)
		return cached
	}

	if isProbing, exists := e.probing[key]; exists && isProbing {
		e.mu.RUnlock()
		log.Printf("[Probe] Probe already in progress for slave %d, type %s, waiting...", slaveID, regType)
		for i := 0; i < 100; i++ {
			time.Sleep(50 * time.Millisecond)
			e.mu.RLock()
			if cached, exists := e.cache[key]; exists {
				e.mu.RUnlock()
				return cached
			}
			if !e.probing[key] {
				e.mu.RUnlock()
				break
			}
			e.mu.RUnlock()
		}
		e.mu.RLock()
		if cached, exists := e.cache[key]; exists {
			e.mu.RUnlock()
			return cached
		}
		e.mu.RUnlock()
		return nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	e.probing[key] = true
	e.mu.Unlock()

	log.Printf("[Probe] Starting probe for slave %d, type %s, range %d-%d", slaveID, regType, startAddr, endAddr)

	result := &DeviceProbeResult{
		SlaveID:       slaveID,
		RegType:       regType,
		RTTModel:      NewRTTModel(),
		LastProbeTime: time.Now(),
	}

	blocks := e.binaryProbe(ctx, slaveID, regType, startAddr, endAddr, 0, 0)
	result.Blocks = e.mergeBlocks(blocks)

	if e.config.EnableMTUProbe {
		e.probeMTU(ctx, slaveID, regType, result)
	}

	e.mu.Lock()
	e.cache[key] = result
	delete(e.probing, key)
	e.mu.Unlock()

	e.persistCache()

	log.Printf("[Probe] Probe completed for slave %d, type %s: %d blocks found", slaveID, regType, len(result.Blocks))
	for _, b := range result.Blocks {
		log.Printf("[Probe]   Valid block: %d-%d", b.Start, b.End)
	}

	return result
}

func (e *ProbeEngine) binaryProbe(ctx context.Context, slaveID uint8, regType string, startAddr uint16, endAddr uint16, depth int, consecutive int) []ValidBlock {
	if depth > e.config.MaxDepth {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	e.transport.SetUnitID(slaveID)

	length := endAddr - startAddr + 1

	// Try to read the entire range
	ok := e.tryRead(ctx, slaveID, regType, startAddr, length)

	if ok {
		return []ValidBlock{{Start: startAddr, End: endAddr}}
	}

	if length == 1 {
		return nil
	}

	if consecutive >= e.config.MaxConsecutive {
		return nil
	}

	// Add a small delay between probe steps to be polite
	time.Sleep(50 * time.Millisecond)

	mid := startAddr + length/2

	leftBlocks := e.binaryProbe(ctx, slaveID, regType, startAddr, mid-1, depth+1, consecutive)
	if len(leftBlocks) == 0 {
		consecutive++
	} else {
		consecutive = 0
	}

	rightBlocks := e.binaryProbe(ctx, slaveID, regType, mid, endAddr, depth+1, consecutive)
	if len(rightBlocks) == 0 {
		consecutive++
	} else {
		consecutive = 0
	}

	return append(leftBlocks, rightBlocks...)
}

func (e *ProbeEngine) tryRead(ctx context.Context, slaveID uint8, regType string, addr uint16, length uint16) bool {
	_, err := e.transport.ReadRegisters(ctx, regType, addr, length)

	if err == nil {
		return true
	}

	// Check if it's an illegal address error
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "illegal") || strings.Contains(errMsg, "exception 2") {
		return false
	}

	// For other errors (timeout, connection issues), return false
	return false
}

func (e *ProbeEngine) mergeBlocks(blocks []ValidBlock) []ValidBlock {
	if len(blocks) == 0 {
		return blocks
	}

	// Sort blocks by start address
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Start < blocks[j].Start
	})

	merged := []ValidBlock{blocks[0]}

	for _, b := range blocks[1:] {
		last := &merged[len(merged)-1]
		if b.Start <= last.End+1 {
			// Merge blocks
			if b.End > last.End {
				last.End = b.End
			}
		} else {
			// Add new block
			merged = append(merged, b)
		}
	}

	return merged
}

func (e *ProbeEngine) probeMTU(ctx context.Context, slaveID uint8, regType string, result *DeviceProbeResult) {
	if len(result.Blocks) == 0 {
		return
	}

	batchSizes := []int{10, 20, 40, 80, 125}
	block := result.Blocks[0]

	for _, size := range batchSizes {
		if block.End-block.Start+1 < uint16(size) {
			continue
		}

		ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
		start := time.Now()
		_, err := e.transport.ReadRegisters(ctx, regType, block.Start, uint16(size))
		rtt := time.Since(start)
		cancel()

		if err == nil {
			result.RTTModel.Record(size, rtt)
		}
	}

	result.MaxBatchSize = result.RTTModel.BestBatchSize()

	log.Printf("[Probe] MTU probe result: optimal batch size = %d", result.MaxBatchSize)
}

func (e *ProbeEngine) GetCachedResult(slaveID uint8, regType string) *DeviceProbeResult {
	key := e.cacheKey(slaveID, regType)
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cache[key]
}

func (e *ProbeEngine) InvalidateCache(slaveID uint8, regType string) {
	key := e.cacheKey(slaveID, regType)
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.cache, key)
	e.persistCache()
	log.Printf("[Probe] Cache invalidated for slave %d, type %s", slaveID, regType)
}

func (e *ProbeEngine) cacheKey(slaveID uint8, regType string) string {
	return fmt.Sprintf("%d:%s", slaveID, regType)
}

func (e *ProbeEngine) persistCache() {
	e.mu.RLock()
	defer e.mu.RUnlock()

	data, err := json.MarshalIndent(e.cache, "", "  ")
	if err != nil {
		zap.L().Error("[Probe] Failed to marshal cache", zap.Error(err))
		return
	}

	if err := os.WriteFile(e.config.PersistPath, data, 0644); err != nil {
		zap.L().Error("[Probe] Failed to persist cache", zap.Error(err))
	}
}

func (e *ProbeEngine) loadPersistedCache() {
	data, err := os.ReadFile(e.config.PersistPath)
	if err != nil {
		if !os.IsNotExist(err) {
			zap.L().Warn("[Probe] Failed to load cache", zap.Error(err))
		}
		return
	}

	if err := json.Unmarshal(data, &e.cache); err != nil {
		zap.L().Warn("[Probe] Failed to unmarshal cache", zap.Error(err))
	}
}

func (e *ProbeEngine) TriggerReprobe(ctx context.Context, slaveID uint8, regType string, startAddr uint16, endAddr uint16) {
	go func() {
		e.InvalidateCache(slaveID, regType)
		result := e.ProbeDevice(ctx, slaveID, regType, startAddr, endAddr)
		zap.L().Info("[Probe] Reprobe completed",
			zap.Uint8("slaveID", slaveID),
			zap.String("regType", regType),
			zap.Int("blockCount", len(result.Blocks)),
		)
	}()
}

type AddressFilter interface {
	Filter(slaveID uint8, regType string, startAddr uint16, endAddr uint16) bool
}

type ValidAddressMap struct {
	engine  *ProbeEngine
	mu      sync.RWMutex
	minAddr map[string]uint16
	maxAddr map[string]uint16
	probing map[string]bool
}

func NewValidAddressMap(engine *ProbeEngine) *ValidAddressMap {
	return &ValidAddressMap{
		engine:  engine,
		minAddr: make(map[string]uint16),
		maxAddr: make(map[string]uint16),
		probing: make(map[string]bool),
	}
}

func (m *ValidAddressMap) cacheKey(slaveID uint8, regType string) string {
	return m.engine.cacheKey(slaveID, regType)
}

func (m *ValidAddressMap) GetValidBlocks(slaveID uint8, regType string, configuredStart uint16, configuredEnd uint16) []ValidBlock {
	cached := m.engine.GetCachedResult(slaveID, regType)
	if cached == nil || len(cached.Blocks) == 0 {
		m.TriggerProbeIfNeeded(slaveID, regType, configuredStart, configuredEnd)
		// Fallback to configured range but limit to reasonable size to avoid timeout degradation
		maxSafeSize := uint16(20) // Default safe batch size
		if configuredEnd-configuredStart+1 > maxSafeSize {
			return []ValidBlock{{Start: configuredStart, End: configuredStart + maxSafeSize - 1}}
		}
		return []ValidBlock{{Start: configuredStart, End: configuredEnd}}
	}

	var filtered []ValidBlock
	for _, block := range cached.Blocks {
		if block.End < configuredStart || block.Start > configuredEnd {
			continue
		}

		start := block.Start
		end := block.End

		if start < configuredStart {
			start = configuredStart
		}
		if end > configuredEnd {
			end = configuredEnd
		}

		if start <= end {
			filtered = append(filtered, ValidBlock{Start: start, End: end})
		}
	}

	if len(filtered) == 0 {
		// No overlap with cache, might be new range
		m.TriggerProbeIfNeeded(slaveID, regType, configuredStart, configuredEnd)
		maxSafeSize := uint16(20)
		if configuredEnd-configuredStart+1 > maxSafeSize {
			return []ValidBlock{{Start: configuredStart, End: configuredStart + maxSafeSize - 1}}
		}
		return []ValidBlock{{Start: configuredStart, End: configuredEnd}}
	}

	return filtered
}

func (m *ValidAddressMap) IsAddressValid(slaveID uint8, regType string, addr uint16) bool {
	cached := m.engine.GetCachedResult(slaveID, regType)
	if cached == nil || len(cached.Blocks) == 0 {
		m.TriggerProbeIfNeeded(slaveID, regType, addr, addr)
		return true
	}

	minBlock := cached.Blocks[0].Start
	maxBlock := cached.Blocks[0].End
	for _, block := range cached.Blocks {
		if block.Start < minBlock {
			minBlock = block.Start
		}
		if block.End > maxBlock {
			maxBlock = block.End
		}
	}

	if addr < minBlock || addr > maxBlock {
		m.TriggerProbeIfNeeded(slaveID, regType, addr, addr)
		return true
	}

	for _, block := range cached.Blocks {
		if addr >= block.Start && addr <= block.End {
			return true
		}
	}
	return false
}

func (m *ValidAddressMap) RecordAddressRange(slaveID uint8, regType string, start uint16, end uint16) {
	key := m.cacheKey(slaveID, regType)
	m.mu.Lock()
	defer m.mu.Unlock()

	if existingStart, exists := m.minAddr[key]; exists {
		if start < existingStart {
			m.minAddr[key] = start
		}
		if end > m.maxAddr[key] {
			m.maxAddr[key] = end
		}
	} else {
		m.minAddr[key] = start
		m.maxAddr[key] = end
	}
}

func (m *ValidAddressMap) TriggerProbeIfNeeded(slaveID uint8, regType string, start uint16, end uint16) {
	key := m.cacheKey(slaveID, regType)

	m.mu.Lock()
	if m.probing[key] {
		m.mu.Unlock()
		return
	}
	m.probing[key] = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.probing, key)
			m.mu.Unlock()
		}()

		cached := m.engine.GetCachedResult(slaveID, regType)
		m.mu.RLock()
		if recordedStart, exists := m.minAddr[key]; exists {
			start = recordedStart
			end = m.maxAddr[key]
		}
		m.mu.RUnlock()

		if cached != nil && len(cached.Blocks) > 0 {
			minBlock := cached.Blocks[0].Start
			maxBlock := cached.Blocks[0].End
			for _, block := range cached.Blocks {
				if block.Start < minBlock {
					minBlock = block.Start
				}
				if block.End > maxBlock {
					maxBlock = block.End
				}
			}

			if start >= minBlock && end <= maxBlock {
				return
			}

			if start > minBlock {
				start = minBlock
			}
			if end < maxBlock {
				end = maxBlock
			}
			// Do NOT invalidate cache here, let ProbeDevice replace it when done
		}

		log.Printf("[ValidAddressMap] Triggering auto-probe for slave %d, type %s, range %d-%d", slaveID, regType, start, end)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		m.engine.ProbeDevice(ctx, slaveID, regType, start, end)
	}()
}

func (m *ValidAddressMap) GetOptimalBatchSize(slaveID uint8, regType string) int {
	cached := m.engine.GetCachedResult(slaveID, regType)
	if cached != nil {
		if cached.MaxBatchSize > 0 {
			return cached.MaxBatchSize
		}
		if cached.RTTModel != nil {
			return cached.RTTModel.BestBatchSize()
		}
	}
	return 40
}
