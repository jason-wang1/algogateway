package RegisterCenter

import (
	"strconv"
	"sync"

	"google.golang.org/grpc/resolver"
)

type serviceResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
	addrs  sync.Map
	nodes  sync.Map

}

func (r *serviceResolver) setAddr(addr string, count int) {
	r.addrs.Store(addr, count)
}

func (r *serviceResolver) addNodes(
	addr string, // ip:port
	idx int,
	rn *RealNode,
) {
	val, ok := r.addrs.Load(addr)
	if !ok {
		return
	}
	if count := val.(int); idx >= count {
		return
	}

	key := addr + ":" + strconv.Itoa(idx)
	vn := &VirtualNode{
		Key: key,
		Rn:  rn,
	}
	address := resolver.Address{Addr: addr}
	address = SetNodeInfo(address, vn)
	r.nodes.Store(key, address)
}

func (r *serviceResolver) delAddr(
	addr string,
) {
	val, ok := r.addrs.Load(addr)
	if !ok {
		return
	}
	count := val.(int)
	for idx := 0; idx < count; idx++ {
		key := addr + ":" + strconv.Itoa(idx)
		r.nodes.Delete(key)
	}
	r.addrs.Delete(addr)

}

func (r *serviceResolver) update() {
	r.cc.UpdateState(resolver.State{Addresses: r.getAddress()})
}

func (r *serviceResolver) getAddress() []resolver.Address {
	var addr []resolver.Address
	r.nodes.Range(func(key, value interface{}) bool {
		addr = append(addr, value.(resolver.Address))
		return true
	})
	return addr
}

func (*serviceResolver) ResolveNow(o resolver.ResolveNowOptions) {}
func (*serviceResolver) Close()                                  {}
