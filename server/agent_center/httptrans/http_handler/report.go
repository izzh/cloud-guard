package http_handler

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/Elkeid/server/agent_center/common"
	"github.com/bytedance/Elkeid/server/agent_center/common/ylog"
	"github.com/bytedance/Elkeid/server/agent_center/grpctrans/grpc_handler"
	"github.com/levigross/grequests"
)

func ReportAgentInfo() {
	reset := false
	ticker := time.NewTicker(60 * time.Second)
	for {
		select {
		case <-ticker.C:
			if !reset {
				ticker.Reset(1 * time.Minute)
				reset = true
			}
			res := grpc_handler.GlobalGRPCPool.GetList()
			for _, v := range res {
				agentInfo := v.GetAgentDetail()
				data := make(map[string]interface{}, 1)
				data["tenantId"] = agentInfo["tenant_id"]
				data["hostId"] = agentInfo["host_id"]
				platformS := ""
				platformVersionS := ""
				archS := ""
				platform, ok := agentInfo["platform"]
				if ok {
					s, ok := platform.(string)
					if ok {
						platformS = s
					}
				}
				platformVersion, ok := agentInfo["platform_version"]
				if ok {
					s, ok := platformVersion.(string)
					if ok {
						platformVersionS = s
					}
				}
				arch, ok := agentInfo["arch"]
				if ok {
					s, ok := arch.(string)
					if ok {
						archS = s
					}
				}
				data["os"] = strings.Join([]string{platformS, platformVersionS, archS}, " ")
				data["cpuCount"] = agentInfo["cpu_count"]
				data["cpuName"] = agentInfo["cpu_name"]
				memSize := 0.0
				memSizeF, ok := agentInfo["total_mem"]
				if ok {
					s, ok := memSizeF.(float64)
					if ok {
						memSize = s / 1024 / 1024 / 1024
					}
				}
				v1 := fmt.Sprintf("%.2fG", memSize)
				data["memSize"] = v1
				diskSize := 0.0
				diskSizeF, ok := agentInfo["disk_count"]
				if ok {
					s, ok := diskSizeF.(float64)
					if ok {
						diskSize = s / 1024 / 1024 / 1024
					}
				}
				v2 := fmt.Sprintf("%.2fG", diskSize)
				data["diskSize"] = v2
				data["state"] = 1
				var bootTime int64
				bootTimeF, ok := agentInfo["boot_time"]
				if ok {
					s, ok := bootTimeF.(float64)
					if ok {
						bootTime = int64(s)
					}
				}
				t := time.Unix(bootTime, 0)
				formattedTime := t.Format("2006-01-02 15:04:05")
				data["startupTime"] = formattedTime
				hostRebootEvents := ""
                                hostRebootEventsS, ok  := agentInfo["host_reboot_events"]
				if ok {
					s, ok := hostRebootEventsS.(string)
					if ok {
						hostRebootEvents = s
					}
				}
				ylog.Infof("host_reboot_events: %s", hostRebootEvents)
				hostRebootEventsInfo := make([]map[string]interface{}, 0, 1)
				if hostRebootEvents != "" {
					entries, err := ParseLastLogs(hostRebootEvents)
					if err == nil {
						for _, entry := range entries {
							em := make(map[string]interface{}, 1)
							em["event"] = entry.Action
							em["eventTime"] = entry.StartTime.Format("2006-01-02 15:04:05")
							em["status"] = ""
							ylog.Infof("host_reboot_events: %s",  em["event"].(string))
							hostRebootEventsInfo = append(hostRebootEventsInfo, em)
						}
					} else {
						ylog.Infof("parse host_reboot_events err: %s", err.Error())
					}
				}
				data["hostRebootEvents"] = hostRebootEventsInfo
				ethsInfo := make([]map[string]interface{}, 0, 8)
				ethInfoList := v.GetEthInfosList()
				for _, v := range ethInfoList {
					ethInfo := make(map[string]interface{}, 1)
					for kk, vv := range v {
						if kk == "eth_name" {
							ethInfo["ethName"] = vv
						} else {
							ethInfo[kk] = vv
						}
					}
					ethsInfo = append(ethsInfo, ethInfo)
				}
				data["ethInfo"] = ethsInfo
				if reportData, err := json.Marshal(data); err == nil {
					ylog.Infof("reportData: %s", string(reportData))
				}
				resp, err := grequests.Post(common.ManagerServer+"/agent/report/heartBeat/basicInfo", &grequests.RequestOptions{
					JSON:           data,
					RequestTimeout: 5 * time.Second,
				})
				if err != nil {
					ylog.Errorf("Report basicInfo fail, err: %s", err.Error())
				} else {
					respData := &struct {
						Status int    `json:"status"`
						Msg    string `json:"msg"`
					}{}
					if err := json.Unmarshal(resp.Bytes(), respData); err == nil {
						var tid int64
						tidI, ok  := agentInfo["tenant_id"]
						if ok {
							s, ok := tidI.(int64)
							if ok {
								tid = s
							}
						}
						ylog.Infof("Report tenantID: %s", strconv.Itoa(int(tid)))
						ylog.Infof("resp.status: %s, resp.msg: %s", strconv.Itoa(respData.Status), respData.Msg)
					}
				}
			}
		}
	}
}
// LastLogEntry represents a single log entry from the 'last -x -time-format=iso' command
type LastLogEntry struct {
	Action        string        `json:"action"`
	Description   string        `json:"description"`
	KernelVersion string        `json:"kernel_version"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration"`
}

// ParseLastLog parses a single line from the 'last -x -time-format=iso' command output
func ParseLastLog(line string) (*LastLogEntry, error) {
	// Remove leading/trailing whitespace
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	// Split the line by multiple spaces to handle variable spacing
	fields := regexp.MustCompile(`\s+`).Split(line, -1)
	if len(fields) < 6 {
		return nil, fmt.Errorf("insufficient fields in line: %s", line)
	}

	entry := &LastLogEntry{}

	// Parse action and description
	// Handle cases where description might contain spaces or parentheses
	actionEnd := 1
	if strings.Contains(fields[0], "(") {
		// Find where the description ends (before kernel version)
		for i, field := range fields {
			if strings.Contains(field, "5.") && strings.Contains(field, "-") {
				actionEnd = i
				break
			}
		}
	} else {
		// For reboot/shutdown, description is typically the second field
		actionEnd = 2
	}

	entry.Action = fields[0]
	if actionEnd > 1 {
		entry.Description = strings.Join(fields[1:actionEnd], " ")
	} else {
		entry.Description = ""
	}

	// Find kernel version (starts with 5. and contains -)
	kernelIndex := -1
	for i, field := range fields {
		if (strings.HasPrefix(field, "5.") || strings.HasPrefix(field, "4.") || strings.HasPrefix(field, "3.") || strings.HasPrefix(field, "6.")) && strings.Contains(field, "-") {
			kernelIndex = i
			break
		}
	}
	if kernelIndex == -1 {
		return nil, fmt.Errorf("kernel version not found in line: %s", line)
	}
	entry.KernelVersion = fields[kernelIndex]

	// Find timestamps (ISO format with + or - timezone)
	timePattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$`)
	var timestamps []string
	for _, field := range fields {
		if timePattern.MatchString(field) {
			timestamps = append(timestamps, field)
		}
	}

	if len(timestamps) < 2 {
		return nil, fmt.Errorf("insufficient timestamps found in line: %s", line)
	}

	// Parse start and end times
	startTime, err := time.Parse("2006-01-02T15:04:05-07:00", timestamps[0])
	if err != nil {
		// Try with + timezone
		startTime, err = time.Parse("2006-01-02T15:04:05+07:00", timestamps[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse start time %s: %v", timestamps[0], err)
		}
	}
	entry.StartTime = startTime

	endTime, err := time.Parse("2006-01-02T15:04:05-07:00", timestamps[1])
	if err != nil {
		// Try with + timezone
		endTime, err = time.Parse("2006-01-02T15:04:05+07:00", timestamps[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse end time %s: %v", timestamps[1], err)
		}
	}
	entry.EndTime = endTime

	// Parse duration from the last field (format: (HH:MM))
	durationPattern := regexp.MustCompile(`\((\d{2}):(\d{2})\)`)
	lastField := fields[len(fields)-1]
	matches := durationPattern.FindStringSubmatch(lastField)
	if len(matches) == 3 {
		hours, _ := strconv.Atoi(matches[1])
		minutes, _ := strconv.Atoi(matches[2])
		entry.Duration = time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute
	} else {
		// If duration parsing fails, calculate from start and end times
		entry.Duration = endTime.Sub(startTime)
	}

	return entry, nil
}

// ParseLastLogs parses multiple lines from the 'last -x -time-format=iso' command output
func ParseLastLogs(input string) ([]*LastLogEntry, error) {
	lines := strings.Split(input, "\n")
	var entries []*LastLogEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		ylog.Infof("Parse last log line: %s", line)
		if line == "" || strings.Contains(line, "runlevel"){
			continue
		}

		entry, err := ParseLastLog(line)
		if err != nil {
			// return nil, fmt.Errorf("error parsing line %d: %v", i+1, err)
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
