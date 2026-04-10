package iotplatform

import (
	"edge-gateway/internal/model"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const channelIDPrefix = "iot-platform-"

// mapProtocol converts platform protocol names to gateway protocol names.
func mapProtocol(platform string) (string, error) {
	switch platform {
	case "modbus_rtu":
		return "modbus-rtu", nil
	case "modbus_tcp":
		return "modbus-tcp", nil
	case "dlt645":
		return "dlt645", nil
	case "opcua":
		return "opc-ua", nil
	case "iec104":
		return "", fmt.Errorf("protocol iec104 is not supported by this gateway")
	default:
		return "", fmt.Errorf("unknown protocol: %s", platform)
	}
}

// MakeChannelID builds the gateway-internal channel ID from a platform channel_id.
func MakeChannelID(platformChannelID int) string {
	return channelIDPrefix + strconv.Itoa(platformChannelID)
}

// IsPlatformChannel checks whether a channel ID was created by the IoT platform.
func IsPlatformChannel(channelID string) bool {
	return strings.HasPrefix(channelID, channelIDPrefix)
}

// mapConnConfig converts platform conn_config to the gateway Channel.Config map.
func mapConnConfig(protocol string, connCfg map[string]any) map[string]any {
	cfg := make(map[string]any)

	switch protocol {
	case "modbus_tcp":
		host, _ := connCfg["host"].(string)
		port := toInt(connCfg["port"])
		cfg["url"] = fmt.Sprintf("tcp://%s:%d", host, port)

	case "modbus_rtu", "dlt645":
		cfg["serial_port"] = connCfg["serial_port"]
		cfg["baud_rate"] = toInt(connCfg["baud_rate"])
		cfg["data_bits"] = toInt(connCfg["data_bits"])
		cfg["stop_bits"] = toInt(connCfg["stop_bits"])
		cfg["parity"] = connCfg["parity"]
		cfg["connectionType"] = "serial"

	case "opcua":
		cfg["url"] = connCfg["endpoint"]
		if u, ok := connCfg["username"]; ok {
			cfg["username"] = u
		}
		if p, ok := connCfg["password"]; ok {
			cfg["password"] = p
		}
	}

	return cfg
}

// mapDeviceConfig converts platform device_addr to the gateway Device.Config map.
func mapDeviceConfig(protocol string, addr map[string]any) map[string]any {
	cfg := make(map[string]any)

	switch protocol {
	case "modbus_rtu":
		cfg["slave_id"] = toInt(addr["slave_id"])
	case "modbus_tcp":
		cfg["slave_id"] = toInt(addr["unit_id"])
	case "dlt645":
		cfg["meter_addr"] = addr["meter_addr"]
	}

	return cfg
}

// mapPoints converts platform points to gateway model.Point slice.
func mapPoints(protocol string, pts []PlatformPoint) []model.Point {
	points := make([]model.Point, 0, len(pts))
	for _, pp := range pts {
		p := model.Point{
			ID:    pp.ModelCode,
			Name:  pp.ModelCode,
			Scale: pp.Scale,
		}
		if p.Scale == 0 {
			p.Scale = 1
		}
		p.Offset = pp.Offset

		switch protocol {
		case "modbus_rtu", "modbus_tcp":
			p.FunctionCode = byte(pp.FunctionCode)
			p.Address = strconv.Itoa(pp.RegisterAddr)
			p.DataType = mapModbusDataType(pp.DataFormat)
			p.WordOrder = pp.ByteOrder
			p.ReadWrite = "R"
			p.RegisterType = modbusRegisterType(pp.FunctionCode)

		case "dlt645":
			p.Address = pp.DataIdentifier
			p.DataType = "float64"
			p.ReadWrite = "R"

		case "opcua":
			p.DataType = "float64"
			p.ReadWrite = "R"
		}

		points = append(points, p)
	}
	return points
}

func mapModbusDataType(format string) string {
	switch format {
	case "int16":
		return "int16"
	case "uint16":
		return "uint16"
	case "int32":
		return "int32"
	case "uint32":
		return "uint32"
	case "float32":
		return "float32"
	case "float64":
		return "float64"
	default:
		return "int16"
	}
}

func modbusRegisterType(fc int) model.RegisterType {
	switch fc {
	case 1:
		return model.RegCoil
	case 2:
		return model.RegDiscreteInput
	case 3:
		return model.RegHolding
	case 4:
		return model.RegInput
	default:
		return model.RegHolding
	}
}

// BuildChannel converts a platform config push into a gateway model.Channel.
func BuildChannel(params *ConfigPushParams) (*model.Channel, error) {
	gwProtocol, err := mapProtocol(params.Channel.Protocol)
	if err != nil {
		return nil, err
	}

	ch := &model.Channel{
		ID:       MakeChannelID(params.Channel.ChannelID),
		Name:     params.Channel.Name,
		Protocol: gwProtocol,
		Enable:   true,
		Config:   mapConnConfig(params.Channel.Protocol, params.Channel.ConnConfig),
		StopChan: make(chan struct{}),
	}

	for _, pd := range params.Devices {
		interval := pd.CollectInterval
		if interval == 0 {
			interval = params.Channel.CollectInterval
		}
		if interval <= 0 {
			interval = 10
		}

		dev := model.Device{
			ID:       pd.DeviceID,
			Name:     pd.DeviceID,
			Enable:   true,
			Interval: model.Duration(time.Duration(interval) * time.Second),
			Config:   mapDeviceConfig(params.Channel.Protocol, pd.DeviceAddr),
			Points:   mapPoints(params.Channel.Protocol, pd.Points),
			StopChan: make(chan struct{}),
		}
		ch.Devices = append(ch.Devices, dev)
	}

	return ch, nil
}

// toInt extracts an int from an any value (JSON numbers decode as float64).
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}
