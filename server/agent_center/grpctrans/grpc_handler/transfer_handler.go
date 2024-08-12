package grpc_handler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bytedance/Elkeid/server/agent_center/common"
	"github.com/bytedance/Elkeid/server/agent_center/common/ylog"
	"github.com/bytedance/Elkeid/server/agent_center/grpctrans/pool"
	pb "github.com/bytedance/Elkeid/server/agent_center/grpctrans/proto"
	"github.com/levigross/grequests"
	"google.golang.org/grpc/peer"
)

type TransferHandler struct{}

var (
	//GlobalGRPCPool is the global grpc connection manager
	GlobalGRPCPool *pool.GRPCPool
)

func InitGlobalGRPCPool() {
	option := pool.NewConfig()
	option.PoolLength = common.ConnLimit
	GlobalGRPCPool = pool.NewGRPCPool(option)
}

func (h *TransferHandler) Transfer(stream pb.Transfer_TransferServer) error {
	//Maximum number of concurrent connections
	if !GlobalGRPCPool.LoadToken() {
		err := errors.New("out of max connection limit")
		ylog.Errorf("Transfer", err.Error())
		return err
	}
	defer func() {
		GlobalGRPCPool.ReleaseToken()
	}()

	//Receive the first packet and get the AgentID
	data, err := stream.Recv()
	if err != nil {
		ylog.Errorf("Transfer", "Transfer error %s", err.Error())
		return err
	}
	agentID := data.AgentID
	tenantAuthCode := data.TenantAuthCode
	// TODO: /agent/host/connAuth
	/*
		{
		  "tenantAuthCode" : "11ea5588-1813-41c5-b2b4-398c14233f66",
		  "agentId" : "11ea5588-1813-41c5-b2b4-398c14233f66",
		  "agentVersion" : "1.1.3",
		  "extIp": "210.158.1.69"
		}
	*/

	//Get the client address
	var tenantID int32
	var hostID int32
	p, ok := peer.FromContext(stream.Context())
	if !ok {
		ylog.Errorf("Transfer", "Transfer error %s", err.Error())
		return err
	}
	addr := p.Addr.String()
	ylog.Infof("Transfer", ">>>>connection addr: %s", addr)
	authData := make(map[string]string, 1)
	authData["tenantAuthCode"] = tenantAuthCode
	authData["agentId"] = agentID
	authData["agentVersion"] = data.Version
	authData["extIp"] = addr
	resp, err := grequests.Post(common.ManagerServer+"/agent/host/connAuth", &grequests.RequestOptions{
		JSON:           authData,
		RequestTimeout: 5 * time.Second,
	})
	if err == nil {
		respAuthData := &struct {
			Status int    `json:"status"`
			Msg    string `json:"msg"`
			Data   struct {
				TenantID int32 `json:"tenantId"`
				HostID   int32 `json:"hostId"`
			} `json:"data"`
		}{}
		if err := json.Unmarshal(resp.Bytes(), respAuthData); err == nil {
			if respAuthData.Status == 200 {
				tenantID = respAuthData.Data.TenantID
				hostID = respAuthData.Data.HostID
				ylog.Infof("Transfer", ">>>>auth succ %s %s", agentID, tenantAuthCode)
			} else {
				ylog.Errorf("Transfer", ">>>>auth fail %s %s", agentID, tenantAuthCode)
			}
		}
	} else {
		ylog.Errorf("Transfer", ">>>>auth fail %s", err.Error())
	}

	//add connection info to the GlobalGRPCPool
	ctx, cancelButton := context.WithCancel(context.Background())
	createAt := time.Now().UnixNano() / (1000 * 1000 * 1000)
	connection := pool.Connection{
		AgentID:        agentID,
		TenantAuthCode: tenantAuthCode,
		TenantID:       tenantID,
		HostID:         hostID,
		SourceAddr:     addr,
		CreateAt:       createAt,
		CommandChan:    make(chan *pool.Command),
		Ctx:            ctx,
		CancelFuc:      cancelButton,
	}
	ylog.Infof("Transfer", ">>>>now set %s %v", agentID, connection)
	err = GlobalGRPCPool.Add(agentID, &connection)
	if err != nil {
		ylog.Errorf("Transfer", "Transfer error %s", err.Error())
		return err
	}
	defer func() {
		ylog.Infof("Transfer", "now delete %s ", agentID)
		GlobalGRPCPool.Delete(agentID)
		releaseAgentHeartbeatMetrics(agentID)
	}()

	//Process the first of data
	handleRawData(data, &connection)

	//Receive data from agent
	go recvData(stream, &connection)
	//Send command to agent
	go sendData(stream, &connection)

	//stop here
	<-connection.Ctx.Done()
	return nil
}

func recvData(stream pb.Transfer_TransferServer, conn *pool.Connection) {
	defer conn.CancelFuc()

	for {
		select {
		case <-conn.Ctx.Done():
			ylog.Errorf("recvData", "the send direction of the tcp is closed, now close the recv direction, %s ", conn.AgentID)
			return
		default:
			data, err := stream.Recv()
			if err != nil {
				ylog.Errorf("recvData", "Transfer Recv Error %s, now close the recv direction of the tcp, %s ", err.Error(), conn.AgentID)
				return
			}
			recvCounter.Inc()
			handleRawData(data, conn)
		}
	}
}

func sendData(stream pb.Transfer_TransferServer, conn *pool.Connection) {
	defer conn.CancelFuc()

	for {
		select {
		case <-conn.Ctx.Done():
			ylog.Infof("sendData", "the recv direction of the tcp is closed, now close the send direction, %s ", conn.AgentID)
			return
		case cmd := <-conn.CommandChan:
			//if cmd is nil, close the connection
			if cmd == nil {
				ylog.Infof("sendData", "get the close signal , now close the send direction of the tcp, %s ", conn.AgentID)
				return
			}
			err := stream.Send(cmd.Command)
			if err != nil {
				ylog.Errorf("sendData", "Send Task Error %s %v ", conn.AgentID, cmd)
				cmd.Error = err
				close(cmd.Ready)
				return
			}
			sendCounter.Inc()
			ylog.Infof("sendData", "Transfer Send %s %v ", conn.AgentID, cmd)
			cmd.Error = nil
			close(cmd.Ready)
		}
	}
}
