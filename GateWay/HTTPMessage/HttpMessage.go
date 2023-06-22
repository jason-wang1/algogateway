package HTTPMessage

import (
	_ "AlgoGateWay/GateWay/docs" // swagger docs
	"GateWayCommon/RegisterCenter"
	"net/http"
	"net/http/pprof"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
)

// layne 20210922
// 注意这里 末尾有/ 和 末尾没有/的区别:
// 注册 "/hello/", 假如我们请求路由"/hello/layne", 那么"/hello/"会被匹配到, 但是这个通常不是我们想要的;
// 注册路由的时候不以"/"结尾, 注册"/hello"而不是"/hello/", 就不会匹配到.

const (
	url_path_hello               = "/hello/"
	url_path_swagger             = "/swagger/"
	url_path_metrics             = "/metrics"
	url_path_debug_pprof         = "/debug/pprof/"
	url_path_debug_pprof_cmdline = "/debug/pprof/cmdline/"
	url_path_debug_pprof_profile = "/debug/pprof/profile/"
	url_path_debug_pprof_symbol  = "/debug/pprof/symbol/"
	url_path_debug_pprof_trace   = "/debug/pprof/trace/"
)

type HttpMessage struct {
	mux       *http.ServeMux
	host      string
	RegCenter *RegisterCenter.RegisterCenter // 注册中心
}

func (httpMsg *HttpMessage) Init(
	addr string,
	RegCenter *RegisterCenter.RegisterCenter,
) bool {
	if RegCenter == nil {
		return false
	}
	httpMsg.host = addr
	httpMsg.RegCenter = RegCenter

	httpMsg.mux = http.NewServeMux()
	httpMsg.mux.HandleFunc(url_path_hello, httpMsg.hello)
	httpMsg.mux.HandleFunc(url_path_debug_pprof, pprof.Index)
	httpMsg.mux.HandleFunc(url_path_debug_pprof_cmdline, pprof.Cmdline)
	httpMsg.mux.HandleFunc(url_path_debug_pprof_profile, pprof.Profile)
	httpMsg.mux.HandleFunc(url_path_debug_pprof_symbol, pprof.Symbol)
	httpMsg.mux.HandleFunc(url_path_debug_pprof_trace, pprof.Trace)

	httpMsg.mux.Handle(url_path_swagger, httpSwagger.Handler())

	httpMsg.init_download()

	return true
}

func (httpMsg *HttpMessage) Handler(
	w http.ResponseWriter,
	r *http.Request,
) {
	httpMsg.mux.ServeHTTP(w, r) //	执行
}

func (httpMsg *HttpMessage) hello(
	w http.ResponseWriter,
	r *http.Request,
) {
	st := time.Now()
	header := http.StatusOK
	responseJson(w, header, 0, "hello", st, &emptyData{})
	return
}
