package api

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rethink-paradigms/mesh/internal/orchestrator"
)

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	resp := StatusResponse{}

	// Daemon info
	uptime := time.Duration(0)
	if !h.cfg.Uptime.IsZero() {
		uptime = time.Since(h.cfg.Uptime)
	}
	resp.Daemon = DaemonStatusInfo{
		Version:   h.cfg.Version,
		UptimeSec: int64(uptime.Seconds()),
		StartTime: h.cfg.Uptime.Format(time.RFC3339),
	}

	// Tier
	resp.Tier = h.cfg.Tier
	if resp.Tier == "" {
		resp.Tier = "lite"
	}

	// Bodies
	resp.Bodies = buildBodiesStatus(h.cfg, r.Context())

	// Ports
	resp.Ports = PortsStatusInfo{}
	if h.cfg.Ingress != nil {
		resp.Ports = PortsStatusInfo{
			PoolStart: 9000,
			PoolEnd:   9999,
		}
	}

	// Ingress
	routeCount := 0
	if h.cfg.Ingress != nil {
		if routes, err := h.cfg.Ingress.ListRoutes(r.Context()); err == nil {
			routeCount = len(routes)
		}
	}
	resp.Ingress = IngressStatusInfo{
		RouteCount: routeCount,
	}

	// Capacity
	resp.Capacity = collectCapacity()

	WriteJSON(w, http.StatusOK, resp)
}

func buildBodiesStatus(cfg RouterConfig, ctx context.Context) BodiesStatusInfo {
	info := BodiesStatusInfo{}
	if cfg.Store == nil {
		return info
	}

	records, err := cfg.Store.ListBodies(ctx)
	if err != nil {
		return info
	}

	info.Total = len(records)
	info.List = make([]BodyStatusItem, 0, len(records))

	for _, rec := range records {
		switch rec.State {
		case orchestrator.StateRunning:
			info.Running++
		case orchestrator.StateStopped, orchestrator.StateStopping:
			info.Stopped++
		case orchestrator.StateError:
			info.Error++
		}
		info.List = append(info.List, BodyStatusItem{
			ID:    rec.ID,
			Name:  rec.Name,
			State: string(rec.State),
		})
	}

	return info
}

func collectCapacity() CapacityStatusInfo {
	capInfo := CapacityStatusInfo{}

	// CPU: no real-time percent without sampling, report 0
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	capInfo.CPUPercent = 0.0
	capInfo.MemoryMBUsed = int64(m.Alloc) / 1024 / 1024
	capInfo.MemoryMBTotal = readSystemMemoryMB()

	// Disk: stat root filesystem
	capInfo.DiskGBUsed, capInfo.DiskGBTotal = readDiskUsage("/")

	return capInfo
}

func readSystemMemoryMB() int64 {
	// Try /proc/meminfo on Linux; gracefully degrade on macOS/others
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			var kb int64
			if _, err := fmt.Sscanf(line, "MemTotal: %d kB", &kb); err == nil {
				return kb / 1024
			}
		}
	}
	return 0
}

func readDiskUsage(path string) (usedGB, totalGB float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	availBytes := stat.Bavail * uint64(stat.Bsize)

	totalGB = float64(totalBytes) / (1024 * 1024 * 1024)
	usedGB = float64(totalBytes-availBytes) / (1024 * 1024 * 1024)
	_ = freeBytes // unused but available for future use

	return usedGB, totalGB
}
