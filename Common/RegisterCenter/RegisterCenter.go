package RegisterCenter

import (
	"GateWayCommon/GateWayProtos"
	"GateWayCommon/logger"
	"bytes"
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	cron "github.com/robfig/cron/v3"
)

type RegisterCenter struct {
	serviceInfo *GateWayProtos.ServiceInfo // 本地服务信息
	clientMaps  map[string]*unifiedClient  // 下级服务管理
	crontab     *cron.Cron                 // 定时任务
}

func newCrontabWithSeconds() *cron.Cron {
	secondParser := cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour |
			cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	return cron.New(cron.WithParser(secondParser), cron.WithChain())
}

func (regCenter *RegisterCenter) Init(
	serviceInfo *GateWayProtos.ServiceInfo,
	regAddrList []string,
) error {
	regCenter.serviceInfo = serviceInfo

	newCustomizeBuilder()

	// 初始化下级服务管理
	regCenter.clientMaps = make(map[string]*unifiedClient)
	for _, relyInfo := range regCenter.serviceInfo.RelyList {
		client := new(unifiedClient)
		if err := client.Init(relyInfo.RelyServiceType, relyInfo.RelySemver); err != nil {
			return err
		}
		regCenter.clientMaps[client.serviceName] = client
	}

	// 添加注册中心客户端
	rc_client := new(unifiedClient)
	if err := rc_client.Init(int32(GateWayProtos.ServiceType_REGISTER_CENTER), "1.0.0"); err != nil {
		return err
	}
	regCenter.clientMaps[rc_client.serviceName] = rc_client

	for _, regAddr := range regAddrList {
		rc_info := &GateWayProtos.ServiceInfo{
			ServiceType:   int32(GateWayProtos.ServiceType_REGISTER_CENTER),
			Semver:        "1.0.0",
			Addr:          regAddr,
			Status:        int32(GateWayProtos.ServiceStatus_Online),
			ServiceWeight: 32,
		}
		regCenter.updateClient(rc_info)
	}

	// 初始化定时器, 添加定时任务
	// 1. Ping 3sec
	regCenter.crontab = newCrontabWithSeconds()
	if _, err := regCenter.crontab.AddFunc("*/3 * * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if err := regCenter.ping(ctx); err != nil {
			logger.Log().WithField("err", err).Warn("RegisterCenter Ping Failed")
		}
	}); err != nil {
		return err
	}

	// 2. Check 30sec
	if _, err := regCenter.crontab.AddFunc("*/30 * * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if err := regCenter.check(ctx); err != nil {
			logger.Log().WithField("err", err).Warn("RegisterCenter Check Failed")
		}
	}); err != nil {
		return err
	}

	return nil
}

// Online 服务上线
func (regCenter *RegisterCenter) Online() error {
	// 请求ctx
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// 服务上线
	if err := regCenter.online(ctx); err != nil {
		return err
	}
	
	regCenter.crontab.Start()
	return nil
}

// Offline 服务下线
func (regCenter *RegisterCenter) Offline() error {
	regCenter.serviceInfo.Status = int32(GateWayProtos.ServiceStatus_Offline)
	regCenter.crontab.Stop()

	// 请求ctx
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// 调用下线
	if err := regCenter.offline(ctx); err != nil {
		return err
	}
	return nil
}

func (regCenter *RegisterCenter) CallService(
	ctx context.Context,
	serviceType int32,
	cmd int32,
	request []byte,
) ([]byte, int32, error) {
	// 根据类型获取服务列表
	serviceName := GateWayProtos.ServiceType(serviceType).String()
	client, ok := regCenter.clientMaps[serviceName]
	if !ok || client == nil {
		errStr := "not found server, please check service and version, serviceType = " + strconv.Itoa(int(serviceType))
		return []byte(errStr), int32(GateWayProtos.ResultType_ERR_NO_Server), nil
	}

	// 调用服务
	return client.callService(ctx, cmd, request)
}

func (regCenter *RegisterCenter) updateClient(
	serviceInfo *GateWayProtos.ServiceInfo,
) error {
	if serviceInfo == nil {
		return errors.New("service info is nil.")
	}

	serviceName := GateWayProtos.ServiceType(serviceInfo.ServiceType).String()
	client, ok := regCenter.clientMaps[serviceName]
	if !ok || client == nil {
		return errors.New("service_type not watch.")
	}

	if err := client.updateAddr(serviceInfo); err != nil {
		logger.Log().WithFields(logger.Fields{
			"service_info": serviceInfo,
			"err":          err,
		}).Warn("client.updateAddr Failed")
		return err
	}
	logger.Log().WithField("service_info", serviceInfo).Debug("client.updateAddr Succ")
	return nil
}

// online 上线
func (regCenter *RegisterCenter) online(
	ctx context.Context,
) error {
	srv_type := GateWayProtos.ServiceType_REGISTER_CENTER
	cmd := GateWayProtos.CmdType_CMD_ONLINE

	// 编码
	sendBuffer, err := proto.Marshal(&GateWayProtos.OnlineRequest{
		ServiceInfo: regCenter.serviceInfo,
	})
	if err != nil {
		return err
	}

	data := getCtxFilter(ctx)
	data[Param_PickType] = string(PickType_ConsistentHash)
	data[Param_PickParam] = regCenter.serviceInfo.Addr
	ctx = BuildCtxFilter(ctx, data)

	// 发送统一请求
	recvBuffer, result, err := regCenter.CallService(ctx, int32(srv_type), int32(cmd), sendBuffer)
	if err != nil {
		return err
	}

	// Online协议直接返回下级服务信息, 故只检查返回码
	if result != int32(GateWayProtos.ResultType_OK) {
		errStr := "Send Online Failed" +
			", errCode = " + strconv.Itoa(int(result)) +
			", errMsg = " + string(recvBuffer)
		return errors.New(errStr)
	}

	// 解码
	response := &GateWayProtos.OnlineReply{}
	err = proto.Unmarshal(recvBuffer, response)
	if err != nil {
		return err
	}

	// 解析下级服务, 依次添加
	for _, watchServiceInfo := range response.WatchList {
		for _, serviceInfo := range watchServiceInfo.ServiceList {
			regCenter.updateClient(serviceInfo)
		}
	}
	return nil
}

// offline 下线
func (regCenter *RegisterCenter) offline(
	ctx context.Context,
) error {
	srv_type := GateWayProtos.ServiceType_REGISTER_CENTER
	cmd := GateWayProtos.CmdType_CMD_OFFLINE

	// 编码
	sendBuffer, err := proto.Marshal(&GateWayProtos.OfflineRequest{
		ServiceInfo: regCenter.serviceInfo,
	})
	if err != nil {
		return err
	}

	data := getCtxFilter(ctx)
	data[Param_PickType] = string(PickType_ConsistentHash)
	data[Param_PickParam] = regCenter.serviceInfo.Addr
	ctx = BuildCtxFilter(ctx, data)

	// 发送请求
	recvBuffer, result, err := regCenter.CallService(ctx, int32(srv_type), int32(cmd), sendBuffer)
	if err != nil {
		return err
	}

	// Offline 协议既需要检查返回码, 也需要检查返回ok
	if result != int32(GateWayProtos.ResultType_OK) ||
		bytes.Compare(recvBuffer, []byte("ok")) != 0 {
		errStr := "Send Offline Failed" +
			", errCode = " + strconv.Itoa(int(result)) +
			", errMsg = " + string(recvBuffer)
		return errors.New(errStr)
	}

	return nil
}

// ping 心跳
func (regCenter *RegisterCenter) ping(
	ctx context.Context,
) error {
	srv_type := GateWayProtos.ServiceType_REGISTER_CENTER
	cmd := GateWayProtos.CmdType_CMD_PING

	// 编码
	sendBuffer, err := proto.Marshal(&GateWayProtos.PingRequest{
		ServiceInfo: regCenter.serviceInfo,
	})
	if err != nil {
		return err
	}

	// 定义分发方案为一致性哈希, 采用本地IP地址计算哈希
	data := getCtxFilter(ctx)
	data[Param_PickType] = PickType_ConsistentHash
	data[Param_PickParam] = regCenter.serviceInfo.Addr
	ctx = BuildCtxFilter(ctx, data)

	// 发送请求, 接收返回数据
	recvBuffer, result, err := regCenter.CallService(ctx, int32(srv_type), int32(cmd), sendBuffer)
	if err != nil {
		return err
	}

	// Ping 协议既需要检查返回码, 也需要检查返回ok
	if result != int32(GateWayProtos.ResultType_OK) ||
		bytes.Compare(recvBuffer, []byte("ok")) != 0 {
		errStr := "Send Ping Failed" +
			", errCode = " + strconv.Itoa(int(result)) +
			", errMsg = " + string(recvBuffer)
		return errors.New(errStr)
	}

	return nil
}

// check 校验
func (regCenter *RegisterCenter) check(
	ctx context.Context,
) error {
	srv_type := GateWayProtos.ServiceType_REGISTER_CENTER
	cmd := GateWayProtos.CmdType_CMD_CHECK

	request := &GateWayProtos.CheckRequest{
		ServiceInfo: regCenter.serviceInfo,
	}
	for _, client := range regCenter.clientMaps {
		if client.serviceType != int32(srv_type) {
			request.WatchList = append(request.WatchList,
				client.getWatchServiceInfo())
		}
	}

	// 编码
	sendBuffer, err := proto.Marshal(request)
	if err != nil {
		return err
	}

	// 定义分发方案为一致性哈希, 采用本地IP地址计算哈希
	data := getCtxFilter(ctx)
	data[Param_PickType] = PickType_ConsistentHash
	data[Param_PickParam] = regCenter.serviceInfo.Addr
	ctx = BuildCtxFilter(ctx, data)

	// 发送请求, 接收返回数据
	recvBuffer, result, err := regCenter.CallService(ctx, int32(srv_type), int32(cmd), sendBuffer)
	if err != nil {
		return err
	}

	// Check协议先检查错误码
	if result != int32(GateWayProtos.ResultType_OK) {
		errStr := "Send Check Failed" +
			", errCode = " + strconv.Itoa(int(result)) +
			", errMsg = " + string(recvBuffer)
		return errors.New(errStr)
	}

	// 校验ok, 若成功则返回, 若不是OK, 则需要解码获取下级服务信息
	if bytes.Compare(recvBuffer, []byte("ok")) == 0 {
		return nil
	}

	// 解码
	response := &GateWayProtos.CheckReply{}
	err = proto.Unmarshal(recvBuffer, response)
	if err != nil {
		return err
	}

	// 更新服务状态
	for _, watchServiceInfo := range response.WatchList {
		for _, serviceInfo := range watchServiceInfo.ServiceList {
			// 添加服务, 更新服务状态
			regCenter.updateClient(serviceInfo)
		}
	}
	return nil
}

func (regCenter *RegisterCenter) OnNotify(
	ctx context.Context,
	req *GateWayProtos.UnifiedRequest,
) (*GateWayProtos.UnifiedResponse, error) {
	resp := &GateWayProtos.UnifiedResponse{
		Cmd:      int32(GateWayProtos.CmdType_CMD_NOTIFY),
		Result:   int32(GateWayProtos.ResultType_OK),
		Response: []byte("ok"),
	}

	request := &GateWayProtos.NotifyRequest{}
	if err := proto.Unmarshal(req.Request, request); err != nil {
		resp.Result = int32(GateWayProtos.ResultType_ERR_Decode_Request)
		resp.Response = []byte("Unmarshal Notify Request err.")
		return resp, err
	} else {
		// 添加服务, 更新服务状态
		regCenter.updateClient(request.ServiceInfo)
		return resp, nil
	}
}

func (regCenter *RegisterCenter) OnHello(
	ctx context.Context,
	req *GateWayProtos.UnifiedRequest,
) (*GateWayProtos.UnifiedResponse, error) {
	return &GateWayProtos.UnifiedResponse{
		Cmd:      int32(GateWayProtos.CmdType_CMD_HELLO),
		Result:   int32(GateWayProtos.ResultType_OK),
		Response: []byte("ok"),
	}, nil
}
