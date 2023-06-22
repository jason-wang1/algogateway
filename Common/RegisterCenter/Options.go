package RegisterCenter

import (
	"context"
	"errors"

	"google.golang.org/grpc/resolver"
)

const (
	// P.s> grpc v1.42.0 及之后版本
	// 大写字符都不支持
	// 下划线也不支持
	RegCenterScheme       = "regcenter.scheme"
	RegCenterLoadBalancer = "regcenter.lb"
	RegCenterContext      = "regcenter.ctx"
)

const (
	Param_CMD       = "cmd"
	Param_PickType  = "pick_type"
	Param_PickParam = "pick_param" // 负载均衡参数(hash_key/addr)

	PickType_ConsistentHash = "consistent_hash" // 一致性哈希
	PickType_RandWeight     = "rand_weight"     // 随机权重
	PickType_SpecifyAddr    = "specify_addr"    // 指定地址
)

var ErrLoadBalancingPolicy = errors.New("LoadBalancingPolicy not supported")
var ErrNotFoundPickType = errors.New("not found pick_type")
var ErrNotFoundPickParam = errors.New("not found pick_param")
var ErrNotFoundConn = errors.New("not found conn")

type RealNode struct {
	ServiceType int32  `json:"service_type"`
	Addr        string `json:"addr"`
	Semver      string `json:"semver"`
	Status      int32  `json:"status"`
}

type VirtualNode struct {
	Key string    `json:"key"` // ip:port:idx
	Rn  *RealNode `json:"real_node"`
}

func BuildCtxFilter(
	ctx context.Context,
	data map[string]string,
) context.Context {
	ctx = context.WithValue(ctx, RegCenterContext, data)
	return ctx
}

func getCtxFilter(ctx context.Context) map[string]string {
	if ctx.Value(RegCenterContext) == nil {
		return map[string]string{}
	}
	return ctx.Value(RegCenterContext).(map[string]string)
}

type attrKey_Info struct{}
type attrKey_Idx struct{}

func SetNodeInfo(
	addr resolver.Address,
	vn *VirtualNode,
) resolver.Address {
	addr.Attributes = addr.Attributes.WithValue(attrKey_Info{}, vn)
	return addr
}

func GetNodeInfo(
	addr resolver.Address,
) *VirtualNode {
	info := addr.Attributes.Value(attrKey_Info{})
	vn, _ := info.(*VirtualNode)
	return vn
}
