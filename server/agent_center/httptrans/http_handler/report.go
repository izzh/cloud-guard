package http_handler

import (
	"encoding/json"
	"fmt"
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
				ticker.Reset(10 * time.Minute)
				reset = true
			}
			res := grpc_handler.GlobalGRPCPool.GetList()
			for _, v := range res {
				agentInfo := v.GetAgentDetail()
				data := make(map[string]interface{}, 1)
				data["tenantId"] = agentInfo["tenant_id"]
				data["hostId"] = agentInfo["host_id"]
				data["os"] = strings.Join([]string{agentInfo["platform"].(string), agentInfo["platform_version"].(string), agentInfo["arch"].(string)}, " ")
				data["cpuCount"] = agentInfo["cpu_count"]
				data["cpuName"] = agentInfo["cpu_name"]
				memSize := agentInfo["total_mem"].(float64) / 1024 / 1024 / 1024
				v1 := fmt.Sprintf("%.2fG", memSize)
				data["memSize"] = v1
				diskSize := agentInfo["disk_count"].(float64) / 1000 / 1000 / 1000
				v2 := fmt.Sprintf("%.2fG", diskSize)
				data["diskSize"] = v2
				data["state"] = 1
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
				resp, err := grequests.Post(common.ManagerServer+"/agent/host/heartBeat/basicInfo", &grequests.RequestOptions{
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
						tid := agentInfo["tenant_id"].(int32)
						ylog.Infof("Report tenantID: %s", strconv.Itoa(int(tid)))
						ylog.Infof("resp.status: %s, resp.msg: %s", strconv.Itoa(respData.Status), respData.Msg)
					}
				}
			}
		}
	}
}
