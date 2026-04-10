package core

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// SystemMetrics holds all gateway-level system metrics.
type SystemMetrics struct {
	// CPU
	CPUUsage float64 `json:"cpu_usage"` // Overall CPU usage percentage (0-100)
	CPUCores int     `json:"cpu_cores"`

	// Memory (whole machine)
	MemoryTotal   uint64  `json:"memory_total"`   // bytes
	MemoryUsed    uint64  `json:"memory_used"`    // bytes
	MemoryPercent float64 `json:"memory_percent"` // 0-100
	MemoryUsageMB float64 `json:"memory_usage"`   // MB – backward compat with old dashboard field

	// Disk
	DiskTotal   uint64  `json:"disk_total"`   // bytes
	DiskUsed    uint64  `json:"disk_used"`    // bytes
	DiskPercent float64 `json:"disk_usage"`   // 0-100 – backward compat field name
	DiskFree    uint64  `json:"disk_free"`    // bytes

	// Go runtime
	GoRoutines  int     `json:"goroutines"`
	GoMemAlloc  float64 `json:"go_mem_alloc"` // MB

	// Uptime
	Uptime       int64  `json:"uptime"`        // seconds since gateway process started
	SystemUptime uint64 `json:"system_uptime"` // seconds since OS boot

	// Network I/O (aggregate across all interfaces)
	NetBytesSent uint64  `json:"net_bytes_sent"`
	NetBytesRecv uint64  `json:"net_bytes_recv"`
	NetSendRate  float64 `json:"net_send_rate"` // bytes/s
	NetRecvRate  float64 `json:"net_recv_rate"` // bytes/s

	// Network interfaces detail
	Interfaces []NetInterfaceInfo `json:"interfaces,omitempty"`

	// Wireless / Cellular (best-effort, Linux only)
	WiFi     *WiFiInfo     `json:"wifi,omitempty"`
	Cellular *CellularInfo `json:"cellular,omitempty"`
}

type NetInterfaceInfo struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
	Up        bool   `json:"up"`
}

type WiFiInfo struct {
	SSID     string  `json:"ssid"`
	Signal   int     `json:"signal"`   // dBm
	Quality  int     `json:"quality"`  // 0-100
	Freq     string  `json:"freq"`
	BitRate  string  `json:"bitrate"`
	Connected bool   `json:"connected"`
}

type CellularInfo struct {
	Operator    string  `json:"operator"`
	Technology  string  `json:"technology"` // 4G/LTE/5G
	SignalDBM   int     `json:"signal_dbm"`
	SignalPct   int     `json:"signal_percent"` // 0-100
	RSSI        int     `json:"rssi"`
	RSRP        int     `json:"rsrp"`
	RSRQ        int     `json:"rsrq"`
	SINR        int     `json:"sinr"`
	IMEI        string  `json:"imei"`
	Connected   bool    `json:"connected"`
}

// SysMonitor collects system metrics periodically and exposes the latest snapshot.
type SysMonitor struct {
	mu        sync.RWMutex
	latest    SystemMetrics
	startTime time.Time

	// previous net counters for rate calculation
	prevBytesSent uint64
	prevBytesRecv uint64
	prevTime      time.Time

	ctx    context.Context
	cancel context.CancelFunc

	// Subscribers notified on each tick
	subsMu sync.RWMutex
	subs   []func(SystemMetrics)
}

func NewSysMonitor() *SysMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &SysMonitor{
		startTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Subscribe registers a callback invoked on every collection tick.
func (sm *SysMonitor) Subscribe(fn func(SystemMetrics)) {
	sm.subsMu.Lock()
	sm.subs = append(sm.subs, fn)
	sm.subsMu.Unlock()
}

// GetMetrics returns the latest collected metrics snapshot.
func (sm *SysMonitor) GetMetrics() SystemMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.latest
}

// Start begins periodic collection at the given interval.
func (sm *SysMonitor) Start(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	// Collect once immediately
	sm.collect()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-sm.ctx.Done():
				return
			case <-ticker.C:
				sm.collect()
			}
		}
	}()
}

func (sm *SysMonitor) Stop() {
	sm.cancel()
}

func (sm *SysMonitor) collect() {
	m := SystemMetrics{}

	// CPU
	if pcts, err := cpu.PercentWithContext(sm.ctx, 0, false); err == nil && len(pcts) > 0 {
		m.CPUUsage = pcts[0]
	}
	if counts, err := cpu.CountsWithContext(sm.ctx, true); err == nil {
		m.CPUCores = counts
	}

	// Memory
	if vm, err := mem.VirtualMemoryWithContext(sm.ctx); err == nil {
		m.MemoryTotal = vm.Total
		m.MemoryUsed = vm.Used
		m.MemoryPercent = vm.UsedPercent
		m.MemoryUsageMB = float64(vm.Used) / 1024 / 1024
	}

	// Disk – use root partition
	rootPath := "/"
	if runtime.GOOS == "windows" {
		rootPath = "C:\\"
	}
	if du, err := disk.UsageWithContext(sm.ctx, rootPath); err == nil {
		m.DiskTotal = du.Total
		m.DiskUsed = du.Used
		m.DiskPercent = du.UsedPercent
		m.DiskFree = du.Free
	}

	// Go runtime
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.GoRoutines = runtime.NumGoroutine()
	m.GoMemAlloc = float64(memStats.Alloc) / 1024 / 1024

	// Uptime
	m.Uptime = int64(time.Since(sm.startTime).Seconds())
	if hostInfo, err := host.InfoWithContext(sm.ctx); err == nil {
		m.SystemUptime = hostInfo.Uptime
	}

	// Network I/O
	if counters, err := net.IOCountersWithContext(sm.ctx, true); err == nil {
		var totalSent, totalRecv uint64
		for _, c := range counters {
			totalSent += c.BytesSent
			totalRecv += c.BytesRecv
		}
		m.NetBytesSent = totalSent
		m.NetBytesRecv = totalRecv

		now := time.Now()
		if !sm.prevTime.IsZero() {
			dt := now.Sub(sm.prevTime).Seconds()
			if dt > 0 {
				m.NetSendRate = float64(totalSent-sm.prevBytesSent) / dt
				m.NetRecvRate = float64(totalRecv-sm.prevBytesRecv) / dt
			}
		}
		sm.prevBytesSent = totalSent
		sm.prevBytesRecv = totalRecv
		sm.prevTime = now

		// Build per-interface info
		addrs, _ := net.InterfacesWithContext(sm.ctx)
		addrMap := make(map[string]string)
		upMap := make(map[string]bool)
		for _, a := range addrs {
			for _, addr := range a.Addrs {
				ip := addr.Addr
				if idx := strings.Index(ip, "/"); idx > 0 {
					ip = ip[:idx]
				}
				if ip != "" && !strings.HasPrefix(ip, "fe80") && ip != "::1" && ip != "127.0.0.1" {
					addrMap[a.Name] = ip
					break
				}
			}
			for _, f := range a.Flags {
				if f == "up" {
					upMap[a.Name] = true
					break
				}
			}
		}
		for _, c := range counters {
			if c.Name == "lo" || strings.HasPrefix(c.Name, "veth") || strings.HasPrefix(c.Name, "docker") || strings.HasPrefix(c.Name, "br-") {
				continue
			}
			m.Interfaces = append(m.Interfaces, NetInterfaceInfo{
				Name:      c.Name,
				IP:        addrMap[c.Name],
				BytesSent: c.BytesSent,
				BytesRecv: c.BytesRecv,
				Up:        upMap[c.Name],
			})
		}
	}

	// WiFi (Linux: iwconfig / nmcli)
	if runtime.GOOS == "linux" {
		m.WiFi = collectWiFiInfo()
		m.Cellular = collectCellularInfo()
	}

	sm.mu.Lock()
	sm.latest = m
	sm.mu.Unlock()

	// Notify subscribers
	sm.subsMu.RLock()
	for _, fn := range sm.subs {
		fn(m)
	}
	sm.subsMu.RUnlock()
}

// collectWiFiInfo tries to read WiFi info via iwconfig or nmcli.
func collectWiFiInfo() *WiFiInfo {
	out, err := exec.Command("sh", "-c", "iwconfig 2>/dev/null | head -20").CombinedOutput()
	if err != nil || len(out) == 0 {
		return nil
	}
	text := string(out)
	if strings.Contains(text, "no wireless extensions") && !strings.Contains(text, "ESSID:") {
		return nil
	}

	info := &WiFiInfo{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "ESSID:") {
			if idx := strings.Index(line, "ESSID:\""); idx >= 0 {
				rest := line[idx+7:]
				if end := strings.Index(rest, "\""); end >= 0 {
					info.SSID = rest[:end]
					info.Connected = info.SSID != "" && info.SSID != "off/any"
				}
			}
		}
		if strings.Contains(line, "Signal level=") {
			fmt.Sscanf(extractAfter(line, "Signal level="), "%d", &info.Signal)
		}
		if strings.Contains(line, "Link Quality=") {
			var num, den int
			fmt.Sscanf(extractAfter(line, "Link Quality="), "%d/%d", &num, &den)
			if den > 0 {
				info.Quality = num * 100 / den
			}
		}
		if strings.Contains(line, "Frequency:") {
			info.Freq = extractField(line, "Frequency:", " ")
		}
		if strings.Contains(line, "Bit Rate=") {
			info.BitRate = extractField(line, "Bit Rate=", " ")
		}
	}
	if !info.Connected && info.SSID == "" {
		return nil
	}
	return info
}

// collectCellularInfo tries to read 4G/LTE modem info via mmcli.
func collectCellularInfo() *CellularInfo {
	// Find first modem index
	out, err := exec.Command("mmcli", "-L").CombinedOutput()
	if err != nil || len(out) == 0 {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	modemIdx := ""
	for _, line := range lines {
		if strings.Contains(line, "/Modem/") {
			parts := strings.Split(line, "/Modem/")
			if len(parts) >= 2 {
				modemIdx = strings.TrimSpace(strings.Split(parts[1], " ")[0])
				break
			}
		}
	}
	if modemIdx == "" {
		return nil
	}

	out, err = exec.Command("mmcli", "-m", modemIdx).CombinedOutput()
	if err != nil {
		return nil
	}
	text := string(out)

	info := &CellularInfo{Connected: strings.Contains(text, "state: 'connected'")}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "operator name:") {
			info.Operator = strings.TrimSpace(strings.TrimPrefix(line, "operator name:"))
		}
		if strings.HasPrefix(line, "access tech:") {
			info.Technology = strings.TrimSpace(strings.TrimPrefix(line, "access tech:"))
		}
		if strings.Contains(line, "signal quality:") {
			fmt.Sscanf(extractAfter(line, "signal quality:"), "%d", &info.SignalPct)
		}
	}

	// Signal details
	out, err = exec.Command("mmcli", "-m", modemIdx, "--signal-get").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "rssi:") {
				fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "rssi:")), "%d", &info.RSSI)
			}
			if strings.HasPrefix(line, "rsrp:") {
				fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "rsrp:")), "%d", &info.RSRP)
			}
			if strings.HasPrefix(line, "rsrq:") {
				fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "rsrq:")), "%d", &info.RSRQ)
			}
			if strings.HasPrefix(line, "s/n:") || strings.HasPrefix(line, "snr:") {
				fmt.Sscanf(strings.TrimSpace(line[strings.Index(line, ":")+1:]), "%d", &info.SINR)
			}
		}
		info.SignalDBM = info.RSRP
	}

	if !info.Connected && info.Operator == "" {
		return nil
	}
	return info
}

func extractAfter(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(s[idx+len(prefix):])
}

func extractField(s, prefix, delim string) string {
	rest := extractAfter(s, prefix)
	if idx := strings.Index(rest, delim); idx > 0 {
		return rest[:idx]
	}
	return rest
}
