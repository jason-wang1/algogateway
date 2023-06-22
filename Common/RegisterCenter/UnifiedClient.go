package RegisterCenter

import (
	"GateWayCommon/GateWayProtos"
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/mod/semver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
)

func newGrpcConn(addr string) (*grpc.ClientConn, error) {
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}

	service_config := fmt.Sprintf(`{"LoadBalancingPolicy": "%s"}`, RegCenterLoadBalancer)

	return grpc.Dial(
		addr,
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(kacp),
		grpc.WithDefaultServiceConfig(service_config),
		grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)),
	)
}

type unifiedClient struct {
	serviceType int32  // 服务类型
	serviceName string // 服务名称
	relySemver  string // 服务依赖版本号

	client          GateWayProtos.UnifiedServiceClient // 服务客户端
	serviceResolver *serviceResolver                   // 服务解析器

	rwlock sync.RWMutex                          // 读写锁
	si_map map[string]*GateWayProtos.ServiceInfo // 客户端信息
	rn_map map[string]*RealNode                  // 真实结点信息
}

func (client *unifiedClient) Init(
	serviceType int32,
	relySemver string,
) error {
	client.serviceType = serviceType
	client.serviceName = GateWayProtos.ServiceType(serviceType).String()

	// 校验版本号
	if relySemver == "" {
		return errors.New("semver is empty")
	}
	if relySemver[0] != 'v' {
		relySemver = "v" + relySemver
	}
	// 无效的semver版本
	if !semver.IsValid(relySemver) {
		return errors.New("semver is invalid")
	}
	client.relySemver = relySemver

	client.rwlock.Lock()
	client.si_map = make(map[string]*GateWayProtos.ServiceInfo)
	client.rn_map = make(map[string]*RealNode)
	client.rwlock.Unlock()

	// 初始化服务解析器
	resolver.Register(client)

	// 初始化客户端连接
	addr := RegCenterScheme + ":///" + client.serviceName
	return client.connection(addr)
}

func (client *unifiedClient) connection(addr string) error {
	if conn, err := newGrpcConn(addr); err != nil {
		return err
	} else {
		client.client = GateWayProtos.NewUnifiedServiceClient(conn)
		return nil
	}
}

func (client *unifiedClient) callService(
	ctx context.Context,
	cmd int32,
	request []byte,
) ([]byte, int32, error) {
	data := getCtxFilter(ctx)
	data[Param_CMD] = strconv.Itoa(int(cmd))
	if _, ok := data[Param_PickType]; !ok {
		data[Param_PickType] = PickType_RandWeight
	}
	ctx = BuildCtxFilter(ctx, data)

	// 调用服务, 失败返回错误信息即可
	if resp, err := client.client.CallService(ctx,
		&GateWayProtos.UnifiedRequest{
			Cmd:     cmd,
			Request: request,
		},
	); err != nil {
		return []byte(""), int32(GateWayProtos.ResultType_ERR_Call_Service), err
	} else {
		return resp.Response, resp.Result, nil
	}
}

func (client *unifiedClient) updateAddr(
	serviceInfo *GateWayProtos.ServiceInfo,
) error {
	if serviceInfo == nil {
		panic(errors.New("service info is nil"))
	}

	rn := &RealNode{
		ServiceType: serviceInfo.ServiceType,
		Addr:        serviceInfo.Addr,
		Semver:      serviceInfo.Semver,
		Status:      serviceInfo.Status,
	}

	// 服务信息写回map
	client.rwlock.Lock()
	client.rn_map[serviceInfo.Addr] = rn
	client.si_map[serviceInfo.Addr] = serviceInfo
	client.rwlock.Unlock()

	// 更新服务地址(上线下线都需要做, 版本不匹配会自动过滤掉)
	if client.serviceResolver == nil {
		return errors.New("service resolver is nil")
	}

	addr := serviceInfo.GetAddr()
	weight := int(serviceInfo.GetServiceWeight())
	if weight <= 0 {
		// 兼容逻辑, 默认权重为32
		weight = 32
	}

	if serviceInfo.Status == int32(GateWayProtos.ServiceStatus_Online) &&
		client.checkVersion(serviceInfo.Semver) {
		client.serviceResolver.setAddr(addr, weight)
		for idx := 0; idx < weight; idx++ {
			client.serviceResolver.addNodes(addr, idx, rn)
		}
	} else {
		client.serviceResolver.delAddr(addr)
	}
	client.serviceResolver.update()
	return nil
}

// checkVersion 注册版本号校验
func (client *unifiedClient) checkVersion(
	version string,
) bool {
	// 判断服务类型
	if version == "" {
		return false
	}

	// 加上前导v
	if version[0] != 'v' {
		version = "v" + version
	}

	// 无效的semver版本
	if !semver.IsValid(version) {
		return false
	}

	// 主版本号应当一致
	if semver.Major(version) != semver.Major(client.relySemver) {
		return false
	}

	// 版本号应大于等于要求的版本号
	if semver.Compare(version, client.relySemver) < 0 {
		return false
	}

	return true
}

func (client *unifiedClient) getWatchServiceInfo() *GateWayProtos.WatchServiceInfo {
	watchServiceInfo := &GateWayProtos.WatchServiceInfo{}
	watchServiceInfo.ServiceType = client.serviceType

	client.rwlock.RLock()
	defer client.rwlock.RUnlock()
	for _, info := range client.si_map {
		watchServiceInfo.ServiceList = append(watchServiceInfo.ServiceList, info)
	}
	return watchServiceInfo
}

func (client *unifiedClient) Build(
	target resolver.Target,
	cc resolver.ClientConn,
	opts resolver.BuildOptions,
) (resolver.Resolver, error) {
	client.serviceResolver = &serviceResolver{
		target: target,
		cc:     cc,
	}
	// 这里的update会一定概率导致服务管理注册失败
	// client.serviceResolver.update()
	return client.serviceResolver, nil
}

func (*unifiedClient) Scheme() string { return RegCenterScheme }
