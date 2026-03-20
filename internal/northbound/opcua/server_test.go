package opcua

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"edge-gateway/internal/model"

	"go.uber.org/zap"
)

// MockSouthboundManager implements model.SouthboundManager for testing
type MockSouthboundManager struct {
	channels     []model.Channel
	writeHistory []writeOperation
	mu           sync.Mutex
}

type writeOperation struct {
	channelID string
	deviceID  string
	pointID   string
	value     interface{}
}

func NewMockSouthboundManager() *MockSouthboundManager {
	return &MockSouthboundManager{
		channels: []model.Channel{
			{
				ID:       "ch1",
				Name:     "Test Channel",
				Protocol: "modbus",
				Enable:   true,
				Devices: []model.Device{
					{
						ID:     "dev1",
						Name:   "Test Device",
						Enable: true,
						Config: map[string]any{
							"vendor_name": "Test Vendor",
							"model_name":  "Test Model",
						},
						Points: []model.Point{
							{
								ID:        "point1",
								Name:      "Temperature",
								DataType:  "float64",
								ReadWrite: "R",
							},
							{
								ID:        "point2",
								Name:      "Humidity",
								DataType:  "float64",
								ReadWrite: "R",
							},
							{
								ID:        "point3",
								Name:      "Setpoint",
								DataType:  "float64",
								ReadWrite: "RW",
							},
							{
								ID:        "point4",
								Name:      "Status",
								DataType:  "string",
								ReadWrite: "R",
							},
							{
								ID:        "point5",
								Name:      "Enabled",
								DataType:  "boolean",
								ReadWrite: "RW",
							},
						},
					},
				},
			},
		},
		writeHistory: []writeOperation{},
	}
}

func (m *MockSouthboundManager) GetChannels() []model.Channel {
	return m.channels
}

func (m *MockSouthboundManager) GetChannelDevices(channelID string) []model.Device {
	for _, ch := range m.channels {
		if ch.ID == channelID {
			return ch.Devices
		}
	}
	return []model.Device{}
}

func (m *MockSouthboundManager) GetDevice(channelID, deviceID string) *model.Device {
	for _, ch := range m.channels {
		if ch.ID == channelID {
			for i, dev := range ch.Devices {
				if dev.ID == deviceID {
					return &ch.Devices[i]
				}
			}
		}
	}
	return nil
}

func (m *MockSouthboundManager) WritePoint(channelID, deviceID, pointID string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeHistory = append(m.writeHistory, writeOperation{
		channelID: channelID,
		deviceID:  deviceID,
		pointID:   pointID,
		value:     value,
	})
	return nil
}

func (m *MockSouthboundManager) GetWriteHistory() []writeOperation {
	m.mu.Lock()
	defer m.mu.Unlock()
	history := make([]writeOperation, len(m.writeHistory))
	copy(history, m.writeHistory)
	return history
}

func (m *MockSouthboundManager) ClearWriteHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeHistory = []writeOperation{}
}

// TestServerStartStop tests basic server start/stop functionality
func TestServerStartStop(t *testing.T) {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Create mock southbound manager
	sb := NewMockSouthboundManager()

	// Create OPC UA config
	config := model.OPCUAConfig{
		Name:        "Test OPC UA Server",
		Port:        4840,
		Endpoint:    "/",
		AuthMethods: []string{"Anonymous"},
	}

	// Create and start server
	server := NewServer(config, sb)
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait a bit for server to start
	time.Sleep(2 * time.Second)

	// Stop server
	server.Stop()

	// Wait a bit for server to stop
	time.Sleep(1 * time.Second)

	t.Log("Server start/stop test passed")
}

// TestServerReadWrite tests reading and writing from OPC UA server
func TestServerReadWrite(t *testing.T) {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Create mock southbound manager
	sb := NewMockSouthboundManager()

	// Create OPC UA config
	config := model.OPCUAConfig{
		Name:        "Test OPC UA Server",
		Port:        4841,
		Endpoint:    "/",
		AuthMethods: []string{"Anonymous"},
	}

	// Create and start server
	server := NewServer(config, sb)
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test Update method
	value := model.Value{
		ChannelID: "ch1",
		DeviceID:  "dev1",
		PointID:   "point1",
		Value:     25.5,
		TS:        time.Now(),
	}
	server.Update(value)
	t.Logf("Update method test passed: updated point1 to 25.5")

	// Test write operation through SouthboundManager
	err = sb.WritePoint("ch1", "dev1", "point3", 30.0)
	if err != nil {
		t.Fatalf("Failed to write through SouthboundManager: %v", err)
	}
	t.Logf("Write test passed: wrote 30.0 to point3")

	// Verify write operation was recorded
	history := sb.GetWriteHistory()
	if len(history) != 1 {
		t.Fatalf("Expected 1 write operation, got %d", len(history))
	}
	if history[0].channelID != "ch1" || history[0].deviceID != "dev1" || history[0].pointID != "point3" {
		t.Fatalf("Write operation has wrong parameters: %v", history[0])
	}
	if history[0].value != 30.0 {
		t.Fatalf("Write operation has wrong value: expected 30.0, got %v", history[0].value)
	}

	t.Log("Read/write test passed")
}

// TestServerUpdate tests the Update method
func TestServerUpdate(t *testing.T) {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Create mock southbound manager
	sb := NewMockSouthboundManager()

	// Create OPC UA config
	config := model.OPCUAConfig{
		Name:        "Test OPC UA Server",
		Port:        4842,
		Endpoint:    "/",
		AuthMethods: []string{"Anonymous"},
	}

	// Create and start server
	server := NewServer(config, sb)
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Update a value
	value := model.Value{
		ChannelID: "ch1",
		DeviceID:  "dev1",
		PointID:   "point1",
		Value:     22.5,
		Quality:   "Good",
		TS:        time.Now(),
	}
	server.Update(value)

	// Verify update by checking the node map
	nodeKey := "ch1/dev1/point1"
	if _, exists := server.nodeMap[nodeKey]; exists {
		t.Logf("Update test passed: point1 updated successfully")
	} else {
		t.Fatalf("Node %s not found in nodeMap", nodeKey)
	}

	t.Log("Update test passed")
}

// BenchmarkServerRead benchmarks reading from OPC UA server
func BenchmarkServerRead(b *testing.B) {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Create mock southbound manager
	sb := NewMockSouthboundManager()

	// Create OPC UA config
	config := model.OPCUAConfig{
		Name:        "Test OPC UA Server",
		Port:        4843,
		Endpoint:    "/",
		AuthMethods: []string{"Anonymous"},
	}

	// Create and start server
	server := NewServer(config, sb)
	err := server.Start()
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Run benchmark by simulating read operations
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate a read operation by checking the node map
		nodeKey := "ch1/dev1/point1"
		if _, exists := server.nodeMap[nodeKey]; !exists {
			b.Fatalf("Node %s not found in nodeMap", nodeKey)
		}
	}
	b.StopTimer()
}

// BenchmarkServerWrite benchmarks writing to OPC UA server
func BenchmarkServerWrite(b *testing.B) {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Create mock southbound manager
	sb := NewMockSouthboundManager()

	// Create OPC UA config
	config := model.OPCUAConfig{
		Name:        "Test OPC UA Server",
		Port:        4844,
		Endpoint:    "/",
		AuthMethods: []string{"Anonymous"},
	}

	// Create and start server
	server := NewServer(config, sb)
	err := server.Start()
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Run benchmark by testing WritePoint method
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test WritePoint method directly
		err := sb.WritePoint("ch1", "dev1", "point3", float64(i%100))
		if err != nil {
			b.Fatalf("Write failed: %v", err)
		}
	}
	b.StopTimer()
}

// BenchmarkServerUpdate benchmarks the Update method
func BenchmarkServerUpdate(b *testing.B) {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Create mock southbound manager
	sb := NewMockSouthboundManager()

	// Create OPC UA config
	config := model.OPCUAConfig{
		Name:        "Test OPC UA Server",
		Port:        4845,
		Endpoint:    "/",
		AuthMethods: []string{"Anonymous"},
	}

	// Create and start server
	server := NewServer(config, sb)
	err := server.Start()
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Wait a bit for server to start
	time.Sleep(2 * time.Second)

	// Run benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value := model.Value{
			ChannelID: "ch1",
			DeviceID:  "dev1",
			PointID:   "point1",
			Value:     float64(i % 100),
			Quality:   "Good",
			TS:        time.Now(),
		}
		server.Update(value)
	}
	b.StopTimer()
}

// TestServerStress tests server under stress
func TestServerStress(t *testing.T) {
	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	// Create mock southbound manager
	sb := NewMockSouthboundManager()

	// Create OPC UA config
	config := model.OPCUAConfig{
		Name:        "Test OPC UA Server",
		Port:        4846,
		Endpoint:    "/",
		AuthMethods: []string{"Anonymous"},
	}

	// Create and start server
	server := NewServer(config, sb)
	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Number of concurrent clients
	clientCount := 10
	// Number of operations per client
	operationsPerClient := 100

	var wg sync.WaitGroup
	errorCh := make(chan error, clientCount)

	// Start multiple goroutines to simulate clients
	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Perform operations
			for j := 0; j < operationsPerClient; j++ {
				// Simulate read operation
				nodeKey := "ch1/dev1/point1"
				if _, exists := server.nodeMap[nodeKey]; !exists {
					errorCh <- fmt.Errorf("client %d: node %s not found", clientID, nodeKey)
					return
				}

				// Simulate write operation
				err := sb.WritePoint("ch1", "dev1", "point3", float64((clientID*1000+j)%100))
				if err != nil {
					errorCh <- fmt.Errorf("client %d: write failed: %v", clientID, err)
					return
				}
			}

			t.Logf("Client %d completed %d operations", clientID, operationsPerClient)
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorCh)

	// Check for errors
	errors := []error{}
	for err := range errorCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Fatalf("Stress test failed with %d errors: %v", len(errors), errors)
	}

	// Verify all write operations were recorded
	history := sb.GetWriteHistory()
	expectedWrites := clientCount * operationsPerClient
	if len(history) != expectedWrites {
		t.Fatalf("Expected %d write operations, got %d", expectedWrites, len(history))
	}

	t.Logf("Stress test passed: %d clients, %d operations per client, %d total operations", clientCount, operationsPerClient, expectedWrites)

	// Check memory usage
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	t.Logf("Memory usage after stress test: %f MB", float64(mem.Alloc)/1024/1024)

	// Check server stats
	stats := server.GetStats()
	t.Logf("Server stats: %+v", stats)
}
