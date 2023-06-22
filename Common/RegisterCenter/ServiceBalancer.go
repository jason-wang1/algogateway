package RegisterCenter

import (
	"GateWayCommon/ConsistentHash"
	"math/rand"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

func newCustomizeBuilder() {
	balancer.Register(base.NewBalancerBuilder(
		RegCenterLoadBalancer,
		&tdPickerBuilder{},
		base.Config{HealthCheck: true}))
	return
}

type tdPickerBuilder struct{}

func (r *tdPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPickerV2(balancer.ErrNoSubConnAvailable)
	}

	tdp := &tdPicker{
		k2vn:      make(map[string]*VirtualNode),
		k2conn:    make(map[string]balancer.SubConn),
		addr2conn: make(map[string]balancer.SubConn),
		hash:      ConsistentHash.New(),
	}

	for conn, ci := range info.ReadySCs {
		vn := GetNodeInfo(ci.Address)
		key := vn.Key
		addr := vn.Rn.Addr

		tdp.k2vn[key] = vn
		tdp.k2conn[key] = conn
		tdp.addr2conn[addr] = conn
		tdp.hash.Add(key)
	}
	if len(tdp.k2conn) == 0 {
		return base.NewErrPickerV2(balancer.ErrNoSubConnAvailable)
	}
	return tdp
}

type tdPicker struct {
	k2vn      map[string]*VirtualNode     // key->虚拟结点
	k2conn    map[string]balancer.SubConn // key->连接
	addr2conn map[string]balancer.SubConn // addr->连接
	hash      *ConsistentHash.Consistent  // 一致性Hash
	mu        sync.Mutex
}

func (p *tdPicker) Pick(pi balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.k2conn) == 0 {
		return balancer.PickResult{}, ErrNotFoundConn
	}

	filter := getCtxFilter(pi.Ctx)
	pick_type := filter[Param_PickType]
	if pick_type == PickType_ConsistentHash {
		return p.PickConsistentHash(pi, filter)
	} else if pick_type == PickType_RandWeight {
		return p.PickRandWeight(pi, filter)
	} else if pick_type == PickType_SpecifyAddr {
		return p.PickSpecifyAddr(pi, filter)
	} else {
		return balancer.PickResult{}, ErrNotFoundPickType
	}
}

func (p *tdPicker) PickConsistentHash(
	pi balancer.PickInfo,
	filter map[string]string,
) (balancer.PickResult, error) {
	hash_key, ok := filter[Param_PickParam]
	if !ok {
		return balancer.PickResult{}, ErrNotFoundPickParam
	}

	// 获取最近3个结点, 防止有不匹配的情况
	keys, err := p.hash.GetN(hash_key, 3)
	if err != nil {
		return balancer.PickResult{}, err
	}

	// 遍历结点, 返回第一个匹配的结点
	for _, key := range keys {
		vn := p.k2vn[key]
		if p.filterNode(vn, getCtxFilter(pi.Ctx)) {
			sc := p.k2conn[key]
			// fmt.Println("[DEBUG] consistent_hash choose filter =", filter, "hash_key =", hash_key, "key =", key)
			return balancer.PickResult{SubConn: sc}, nil
		}
	}
	return balancer.PickResult{}, ErrNotFoundConn
}

func (p *tdPicker) PickRandWeight(
	pi balancer.PickInfo,
	filter map[string]string,
) (balancer.PickResult, error) {
	var subConns []balancer.SubConn
	for key, conn := range p.k2conn {
		vn := p.k2vn[key]
		if p.filterNode(vn, getCtxFilter(pi.Ctx)) {
			subConns = append(subConns, conn)
		}
	}
	if len(subConns) == 0 {
		return balancer.PickResult{}, ErrNotFoundConn
	}
	// 从满足条件的地址选择一个
	index := rand.Intn(len(subConns))
	sc := subConns[index]
	return balancer.PickResult{SubConn: sc}, nil
}

func (p *tdPicker) PickSpecifyAddr(
	pi balancer.PickInfo,
	filter map[string]string,
) (balancer.PickResult, error) {
	if addr, ok := filter[Param_PickParam]; ok {
		sc := p.addr2conn[addr]
		return balancer.PickResult{SubConn: sc}, nil
	}
	return balancer.PickResult{}, ErrNotFoundPickParam
}

func (p *tdPicker) filterNode(
	vn *VirtualNode,
	filter map[string]string,
) bool {
	// node在线
	// 版本匹配
	// 接口在线
	// 接口限流
	// 接口熔断
	return true
}
