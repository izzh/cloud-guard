package main

import (
	"strconv"
	"time"

	"github.com/bytedance/Elkeid/plugins/collector/engine"
	"github.com/bytedance/Elkeid/plugins/collector/process"
	"github.com/bytedance/Elkeid/plugins/collector/utils"
	plugins "github.com/bytedance/plugins"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

type ProcessHandler struct{}

func (h ProcessHandler) Name() string {
	return "process"
}
func (h ProcessHandler) DataType() int {
	return 5050
}

func (h *ProcessHandler) Handle(c *plugins.Client, cache *engine.Cache, seq string) {
	procs, err := process.Processes(false)
	if err != nil {
		zap.S().Error(err)
	} else {
		currentTime := time.Now().Unix()
                formattedCurrent := utils.FormatTimestamp(currentTime)
		rec := &plugins.Record{
                    DataType:  50501,
                    Timestamp: time.Now().Unix(),
                    Data: &plugins.Payload{
                        Fields: make(map[string]string, 3),
                    },
                }
		rec.Data.Fields["seq"] = formattedCurrent
		if cpuPercents, err := cpu.Percent(0, false); err == nil && len(cpuPercents) != 0 {
		    rec.Data.Fields["cpu_usage"] = strconv.FormatFloat(cpuPercents[0]/100, 'f', 8, 64)
		}
	        if mem, err := mem.VirtualMemory(); err == nil {
		    rec.Data.Fields["mem_usage"] = strconv.FormatFloat(mem.UsedPercent/100, 'f', 8, 64)
	        }
	        c.SendRecord(rec)
		for _, p := range procs {
			time.Sleep(process.TraversalInterval)
			cmdline, err := p.Cmdline()
			if err != nil {
				continue
			}
			stat, err := p.Stat()
			if err != nil {
				continue
			}
			status, err := p.Status()
			if err != nil {
				continue
			}
			ns, _ := p.Namespaces()
			rec := &plugins.Record{
				DataType:  int32(h.DataType()),
				Timestamp: time.Now().Unix(),
				Data: &plugins.Payload{
					Fields: make(map[string]string, 43),
				},
			}
			rec.Data.Fields["seq"] = formattedCurrent
			rec.Data.Fields["cmdline"] = cmdline
			rec.Data.Fields["cwd"], _ = p.Cwd()
			rec.Data.Fields["checksum"], _ = p.ExeChecksum()
			rec.Data.Fields["exe_hash"], _ = p.ExeHash()
			rec.Data.Fields["exe"], _ = p.Exe()
			rec.Data.Fields["pid"] = p.Pid()
			pid, _ := strconv.Atoi(p.Pid())
			cpu, mem, _, _, _, _, _ := process.GetProcResouce(pid)
                        cpuUsage := strconv.FormatFloat(cpu, 'f', 6, 64)
			rec.Data.Fields["cpu"] = cpuUsage
			memUsage := strconv.FormatUint(mem, 10)
			rec.Data.Fields["mem"] = memUsage
			mapstructure.Decode(stat, &rec.Data.Fields)
			mapstructure.Decode(status, &rec.Data.Fields)
			mapstructure.Decode(ns, &rec.Data.Fields)
			m, _ := cache.Get(5056, ns.Pid)
			rec.Data.Fields["container_id"] = m["container_id"]
			rec.Data.Fields["container_name"] = m["container_name"]
			rec.Data.Fields["integrity"] = "true"
			// only for host files
			if _, ok := cache.Get(5057, rec.Data.Fields["exe"]); ok && rec.Data.Fields["container_id"] == "" {
				rec.Data.Fields["integrity"] = "false"
			}
			rec.Data.Fields["package_seq"] = seq
			c.SendRecord(rec)
		}
	}
}
