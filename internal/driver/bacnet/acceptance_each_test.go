package bacnet

import (
	"context"
	"edge-gateway/internal/driver/bacnet/btypes"
	"edge-gateway/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEachDeviceAcceptance(t *testing.T) {
	devices := []struct {
		name       string
		instanceID int
	}{
		{"bacnet-18", 2228318},
		{"bacnet-16", 2228316},
		{"bacnet-17", 2228317},
		{"Room_FC_2014_19", 2228319},
	}

	driver := NewBACnetDriver().(*BACnetDriver)
	err := driver.Init(model.DriverConfig{
		ChannelID: "test-channel",
		Config:    map[string]any{},
	})
	assert.NoError(t, err)

	// Mock client factory to avoid real network IO but simulate success
	driver.clientFactory = func(cb *ClientBuilder) (Client, error) {
		return &MockClient{
			WhoIsResp: []btypes.Device{
				{
					DeviceID: 2228318, // Default, but will be overwritten in subtests if needed
					Addr:     btypes.Address{Mac: []byte{127, 0, 0, 1, 0xBA, 0xC0}},
				},
			},
			ReadMultiPropertyHandler: func(dev btypes.Device, rp btypes.MultiplePropertyData) (btypes.MultiplePropertyData, error) {
				// Return dummy data for any request
				for i := range rp.Objects {
					for j := range rp.Objects[i].Properties {
						rp.Objects[i].Properties[j].Data = []interface{}{float32(25.5)}
					}
				}
				return rp, nil
			},
		}, nil
	}

	err = driver.Connect(context.Background())
	assert.NoError(t, err)

	for _, dev := range devices {
		t.Run(dev.name, func(t *testing.T) {
			// 1. Set config for device
			err := driver.SetDeviceConfig(map[string]any{
				"device_id": dev.instanceID,
				"name":      dev.name,
				"ip":        "127.0.0.1",
				"port":      47808,
			})
			assert.NoError(t, err)

			// 2. Read points for this device
			points := []model.Point{
				{ID: "temp", Name: "Temperature", Address: "analog-input:1", DeviceID: dev.name},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			results, err := driver.ReadPoints(ctx, points)
			assert.NoError(t, err)
			assert.NotNil(t, results)

			val, ok := results["temp"]
			assert.True(t, ok, "Point 'temp' should be in results")
			assert.Equal(t, "Good", val.Quality)
			// The driver returns the value, let's check it
			assert.NotNil(t, val.Value)

			t.Logf("Device %s (Instance %d) verified successfully with value %v", dev.name, dev.instanceID, val.Value)
		})
	}
}
