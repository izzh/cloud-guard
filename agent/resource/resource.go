package resource

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	cpuName, hostSerial, hostID, hostModel, hostVendor string
	bootTime, _                                        = host.BootTime()
	ioCache, _                                         = lru.New(100)
	procCache, _                                       = lru.New(100)
)

func init() {
	if s, err := cpu.Info(); err == nil && len(s) > 0 {
		cpuName = s[0].ModelName
	}
}

type ProcInfo struct {
	CPUPercent float64
	FdCnt      uint64
	RSS        uint64  //bytes
	ReadSpeed  float64 //Bps
	WriteSpeed float64 //Bps
	StartedAt  int64   //unix timestamp
}

type IOState struct {
	Time       time.Time
	ReadBytes  uint64
	WriteBytes uint64
}

func GetDirSize(path string, except string) uint64 {
	var dirSize uint64 = 0
	readSize := func(path string, file os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !file.IsDir() {
			dirSize += uint64(file.Size())
		} else {
			if file.Name() == except {
				return filepath.SkipDir
			}
		}
		return nil
	}

	filepath.Walk(path, readSize)

	return dirSize
}

func GetBootTime() uint64 {
	return bootTime
}

func GetProcResouce(pid int) (cpu float64, rss uint64, readSpeed, writeSpeed float64, fds int32, startAt int64, err error) {
	var p *process.Process
	if iface, ok := procCache.Get(pid); ok {
		p = iface.(*process.Process)
	} else {
		p, err = process.NewProcess(int32(pid))
		if err != nil {
			return
		}
		procCache.Add(pid, p)
	}
	cpu, _ = p.Percent(0)
	cpu = cpu / 100.0
	startAt, _ = p.CreateTime()
	startAt = startAt / 1000
	fds, _ = p.NumFDs()

	memoryInfo, err := p.MemoryInfo()
	if err == nil {
		rss = memoryInfo.RSS
	}
	io, err := p.IOCounters()
	if err == nil {
		now := time.Now()
		var state IOState
		if stateI, ok := ioCache.Get(pid); ok {
			state = stateI.(IOState)
			readSpeed = float64(io.ReadBytes-state.ReadBytes) / now.Sub(state.Time).Seconds()
			writeSpeed = float64(io.WriteBytes-state.WriteBytes) / now.Sub(state.Time).Seconds()
		} else {
			state = IOState{}
			readSpeed = float64(io.ReadBytes-state.ReadBytes) / (float64(now.Unix() - startAt))
			writeSpeed = float64(io.WriteBytes-state.WriteBytes) / (float64(now.Unix() - startAt))
		}
		state.ReadBytes = io.ReadBytes
		state.WriteBytes = io.WriteBytes
		state.Time = now
		ioCache.Add(pid, state)
	}
	return
}
func GetMemTotal() (total uint64) {
	var m *mem.VirtualMemoryStat
	m, err := mem.VirtualMemory()
	if err == nil {
		total = m.Total
	}
	return
}
func GetCPUName() string {
	return cpuName
}
func GetCPUCounts() (int, error) {
	return cpu.Counts(false)
}
func GetHostInfo() (serial, id, model, vendor string) {
	return hostSerial, hostID, hostModel, hostVendor
}
func GetDiskTotal(path string) (uint64, float64, error) {
	if us, err := disk.Usage(path); err == nil {
		return us.Total, us.UsedPercent, nil
	}
	return 0, 0.0, errors.New("not found")
}
func GetDiskIO() uint64 {
	disks, err := disk.Partitions(false)
	if err != nil {
		return 0
	}
	for _, dev := range disks {
		if strings.Contains(dev.Device, "loop") {
			continue
		}
		stats, err := disk.IOCounters(dev.Device)
		if err == nil {
			for _, stat := range stats {
				return stat.IopsInProgress
			}
		}
	}
	return 0
}

func GetInterfaces() (net.InterfaceStatList, error) {
	return net.Interfaces()
}
