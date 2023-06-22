package HTTPMessage

import (
	"GateWayCommon/AddrLimiter"
	"GateWayCommon/GateWayProtos"
	"GateWayCommon/RegisterCenter"
	"GateWayCommon/jsonpb"
	"GateWayCommon/logger"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	proto "github.com/golang/protobuf/proto"
	protoV2 "google.golang.org/protobuf/proto"
)

// Strval 获取变量的字符串值
// 浮点型 3.0将会转换成字符串3, "3"
// 非数值或字符类型的变量将会被转换成JSON格式字符串
func val2str(value interface{}) string {
	if value == nil {
		return ""
	}
	switch value.(type) {
	case float64:
		return strconv.FormatFloat(value.(float64), 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value.(float32)), 'f', -1, 64)
	case int:
		return strconv.Itoa(value.(int))
	case uint:
		return strconv.Itoa(int(value.(uint)))
	case int8:
		return strconv.Itoa(int(value.(int8)))
	case uint8:
		return strconv.Itoa(int(value.(uint8)))
	case int16:
		return strconv.Itoa(int(value.(int16)))
	case uint16:
		return strconv.Itoa(int(value.(uint16)))
	case int32:
		return strconv.Itoa(int(value.(int32)))
	case uint32:
		return strconv.Itoa(int(value.(uint32)))
	case int64:
		return strconv.FormatInt(value.(int64), 10)
	case uint64:
		return strconv.FormatUint(value.(uint64), 10)
	case string:
		return value.(string)
	case []byte:
		return string(value.([]byte))
	default:
		newValue, _ := json.Marshal(value)
		return string(newValue)
	}
}

// 获取正在运行的函数名
func get_func_name() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	name := f.Name()
	return name
}

// Convert json to map[string]string
func json2KVMap(byte_json []byte) (map[string]string, error) {
	// json 2 map[string]interface{}
	kimap := make(map[string]interface{})
	if err := json.Unmarshal(byte_json, &kimap); err != nil {
		return nil, err
	}

	// map[string]interface{} 2 map[string]string
	kvmap := make(map[string]string)
	for k, v := range kimap {
		kvmap[k] = val2str(v)
	}
	return kvmap, nil
}

func getQueryKVMap(r *http.Request) map[string]string {
	// 从Query读取数据
	var kvmap map[string]string = make(map[string]string)
	query_values := r.URL.Query()
	for k, v := range query_values {
		kvmap[k] = v[0]
	}
	return kvmap
}

// func getFormKVMap(r *http.Request) map[string]string {
// 	// 从Form读取数据
// 	var kvmap map[string]string = make(map[string]string)
// 	for k, v := range r.Form {
// 		kvmap[k] = v[0]
// 	}
// 	return kvmap
// }

func read_body(r *http.Request) []byte {
	body, _ := ioutil.ReadAll(r.Body)
	r.Body.Close() //  must close
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	return body
}

func getBodyKVMap(r *http.Request) (map[string]string, error) {
	// 读取body, 将Json数据转为 kvMap
	kvmap, err := json2KVMap(read_body(r))
	if err != nil {
		return nil, err
	}
	return kvmap, nil
}

func getKVMap(r *http.Request, pt string) (map[string]string, error) {
	if pt == "query" {
		return getQueryKVMap(r), nil
	} else if pt == "body" {
		return getBodyKVMap(r)
	} else {
		return nil, errors.New("ParamType not support, check gateway code.")
	}
}

// HTTPMessage Json Response
type jsonResponse struct {
	Code int32       `json:"code"`
	Msg  string      `json:"msg"`
	Dur  float64     `json:"dur"`
	Data interface{} `json:"data"`
}
type emptyData struct{}

// pb2json_raw 将Proto结构编码为 json.RawMessage
func pb2jsonRaw(
	protoMessage protoV2.Message,
) (json.RawMessage, error) {
	jsonpbMarshaler := &jsonpb.Marshaler{
		EnumsAsInts:  true, // 是否将枚举值设定为整数，而不是字符串类型
		EmitDefaults: true, // 是否将字段值为空的渲染到JSON结构中
		OrigName:     true, // 是否使用原生的proto协议中的字段
	}

	// ProtoV2 to ProtoV1 to json.RawMessage
	data, err := jsonpbMarshaler.MarshalToString(proto.MessageV1(protoMessage))
	if err != nil {
		return nil, err
	}

	// 将Json作为RawMessage返回
	return json.RawMessage(data), nil
}

// responseJson 返回Json格式信息
func responseJson(
	w http.ResponseWriter,
	header int,
	code int32,
	msg string,
	st time.Time,
	data interface{},
) {
	dur := time.Since(st).Seconds()
	jsonResponse := &jsonResponse{
		Code: code,
		Msg:  msg,
		Dur:  dur,
		Data: data,
	}

	// Data应保证不为nil, 否则返回的Json不符合要求.
	if jsonResponse.Data == nil {
		jsonResponse.Data = &emptyData{}
	}

	response, err := json.Marshal(jsonResponse)
	if err != nil {
		header = http.StatusInternalServerError
		w.Header().Set("header", strconv.Itoa(header))
		w.WriteHeader(header)
		w.Write([]byte("{\"code\":1,\"msg\":\"jsonResponse Marshal Json error.\",\"dur\":0.0,\"data\":{}}"))
		return
	}

	w.Header().Set("header", strconv.Itoa(header))
	w.WriteHeader(header)
	w.Write(response)
}

// responseError 返回Json结构
func responseError(
	w http.ResponseWriter,
	header int,
	code int32,
	msg string,
	st time.Time,
) {
	responseJson(w, header, code, msg, st, &emptyData{})
}

// responseProto 将Proto结构作为data返回
func responseProto(
	w http.ResponseWriter,
	header int,
	code int32,
	msg string,
	st time.Time,
	data protoV2.Message,
) {
	if data == nil {
		responseJson(w, header, code, msg, st, &emptyData{})
		return
	}

	json_raw, err := pb2jsonRaw(data)
	if err != nil {
		header = http.StatusInternalServerError
		result := int32(GateWayProtos.ResultType_ERR_Encode_Response)
		responseError(w, header, result, err.Error(), st)
		return
	}
	responseJson(w, header, code, msg, st, json_raw)
}

type requestParam struct {
	FuncName    string `json:"func_name,omitempty"`    // 请求来源函数
	Method      string `json:"method,omitempty"`       // 允许请求方法
	ParamType   string `json:"param_type,omitempty"`   // 读取参数类型
	ServiceType int32  `json:"service_type,omitempty"` // 服务类型
	CMD         int32  `json:"cmd,omitempty"`          // 服务接口
}

// Token校验 - API Token Check 版本
type CheckToken int

const (
	CheckToken_None CheckToken = 0
)

// 负载均衡策略 - Load Balancing Policy
type LBPolicy string

const (
	LBPolicy_RandWeight     LBPolicy = LBPolicy(RegisterCenter.PickType_RandWeight)
	LBPolicy_SpecifyAddr    LBPolicy = LBPolicy(RegisterCenter.PickType_SpecifyAddr)
	LBPolicy_ConsistentHash LBPolicy = LBPolicy(RegisterCenter.PickType_ConsistentHash)
)

// 兜底函数, 成功返回nil, 失败返回错误信息
type GroundRulesFunc func() error

// 获取负载均衡Key函数
type GetLBKeyFunc func() string

type requestOption struct {
	Timeout       int64           `json:"timeout,omitempty"`        // 超时时间, 单位ms; 默认3s超时
	CheckToken    CheckToken      `json:"check_token,omitempty"`    // Token校验
	CheckIP       bool            `json:"check_ip,omitempty"`       // IP白名单校验
	LBPolicy      LBPolicy        `json:"lb_policy,omitempty"`      // 负载均衡策略
	GroundRules   bool            `json:"ground_rules,omitempty"`   // 启用兜底方案
	RequestProto  protoV2.Message `json:"request_proto,omitempty"`  // 请求Proto
	ResponseProto protoV2.Message `json:"response_proto,omitempty"` // 返回Proto, nil为Json返回

	groundRulesFunc GroundRulesFunc
	getLBKeyFunc    GetLBKeyFunc
}

type RequestOption interface {
	apply(*requestOption)
}

type funcOption struct {
	f func(*requestOption)
}

func (fo *funcOption) apply(ro *requestOption) {
	fo.f(ro)
}

func newFuncOption(f func(*requestOption)) *funcOption {
	return &funcOption{
		f: f,
	}
}

//默认参数
func defaultOptions() *requestOption {
	return &requestOption{
		Timeout:         3000,                // 默认3s超时
		CheckToken:      CheckToken_None,     // Token校验方案
		CheckIP:         false,               // 校验请求来源IP
		LBPolicy:        LBPolicy_RandWeight, // 默认使用随机负载均衡
		GroundRules:     false,               // 默认不启用兜底方案
		groundRulesFunc: nil,                 // 兜底方案
		getLBKeyFunc:    nil,                 // 获取负载均衡Key
		RequestProto:    nil,
		ResponseProto:   nil,
	}
}

// withTimeout 超时设置, 单位ms
func withTimeout(timeout int64) RequestOption {
	return newFuncOption(func(o *requestOption) {
		o.Timeout = timeout
	})
}

// // 检查token
// func withCheckToken(version CheckToken) RequestOption {
// 	return newFuncOption(func(o *requestOption) {
// 		o.CheckToken = version
// 	})
// }

// // 检查请求来源IP地址在白名单中
// func withCheckIP() RequestOption {
// 	return newFuncOption(func(o *requestOption) {
// 		o.CheckIP = true
// 	})
// }

// // 请求负载均衡策略
// func withLBPolicy(p LBPolicy, f GetLBKeyFunc) RequestOption {
// 	return newFuncOption(func(o *requestOption) {
// 		o.LBPolicy = p
// 		o.getLBKeyFunc = f
// 	})
// }

// 发送请求Proto
func withRequestProto(proto protoV2.Message) RequestOption {
	return newFuncOption(func(o *requestOption) {
		o.RequestProto = proto
	})
}

// 接收返回Proto, 如果不设置则视为Json返回.
func withResponseProto(proto protoV2.Message) RequestOption {
	return newFuncOption(func(o *requestOption) {
		o.ResponseProto = proto
	})
}

// func withGroundRules(f GroundRulesFunc) RequestOption {
// 	return newFuncOption(func(o *requestOption) {
// 		if f != nil {
// 			o.GroundRules = true
// 			o.groundRulesFunc = f
// 		}
// 	})
// }

// GetClientIP 获取HTTP请求真实客户端IP地址
//	X-Real-IP:
//		只包含客户端机器的一个IP, 如果为空, 某些代理服务器(Nginx)会填充此header.
//	X-Forwarded-For: client1, proxy1, proxy2, proxy3
//		一系列的IP地址列表, 通过一个 逗号+空格(", ") 把多个IP地址区分开,
//		左边(client1)是最原始客户端的IP地址,
//		代理服务器每成功收到一个请求, 就把请求来源IP地址添加到右边.
//	RemoteAddr:
//		包含客户端的'真实IP地址',
//		这是Web服务器从其接收连接并将响应发送到的实际物理IP地址.
//		如果客户端通过代理连接, 它将提供'代理的IP地址'.
func GetClientIP(r *http.Request) (string, error) {
	ip := r.Header.Get("X-Real-IP")
	if net.ParseIP(ip) != nil {
		return ip, nil
	}

	xff := r.Header.Get("X-Forwarded-For")
	for _, xff_ip := range strings.Split(xff, ",") {
		i := strings.TrimSpace(xff_ip)
		if net.ParseIP(i) != nil {
			return i, nil
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	if net.ParseIP(ip) != nil {
		return ip, nil
	}

	return "", errors.New("no valid ip found")
}

func (httpMsg *HttpMessage) common_request_v3(
	w http.ResponseWriter,
	r *http.Request,
	req_param *requestParam,
	opts ...RequestOption,
) error {
	st := time.Now()

	// 循环调用opts获取参数
	req_opts := defaultOptions()
	for _, opt := range opts {
		opt.apply(req_opts)
	}

	// 判断基本参数正常
	if r == nil || req_param == nil || httpMsg.RegCenter == nil {
		header := http.StatusInternalServerError
		code := int32(GateWayProtos.ResultType_ERR_Unknown)
		err := errors.New("http.r or httpMsg.RegCenter or req_param is nil")
		responseError(w, header, code, err.Error(), st)
		return err
	}

	// 检查请求类型是否满足
	if r.Method != req_param.Method {
		header := http.StatusMethodNotAllowed
		code := int32(GateWayProtos.ResultType_ERR_Decode_Request)
		msg := "method not allowed"
		logger.Log().WithFields(logger.Fields{
			"http.Request": logger.Fields{
				"Header": r.Header,
				"Method": r.Method,
				"Host":   r.Host,
				"URL":    r.URL.String(),
			},
			"req.param": req_param,
			"req.opts":  req_opts,
		}).Warn(msg)
		responseError(w, header, code, msg, st)
		return errors.New(msg)
	}

	// 获取客户端真实IP地址
	client_ip, err := GetClientIP(r)
	if err != nil {
		header := http.StatusBadRequest
		code := int32(GateWayProtos.ResultType_ERR_Decode_Request)
		logger.Log().WithFields(logger.Fields{
			"http.Request": logger.Fields{
				"ClientIP": client_ip,
				"Method":   r.Method,
				"Host":     r.Host,
				"URL":      r.URL.String(),
				"Header":   r.Header, // ip 获取失败, 打印Header, 方便查日志
			},
			"req.param": req_param,
			"req.opts":  req_opts,
		}).Warn(err)
		responseError(w, header, code, err.Error(), st)
		return err
	}

	// ip 白名单校验
	if req_opts.CheckIP {
		if check, err := AddrLimiter.IPEnable(client_ip); !check {
			header := http.StatusForbidden
			code := int32(GateWayProtos.ResultType_ERR_Decode_Request)
			logger.Log().WithFields(logger.Fields{
				"http.Request": logger.Fields{
					"ClientIP": client_ip,
					"Method":   r.Method,
					"Host":     r.Host,
					"URL":      r.URL.String(),
					"Header":   r.Header, // ip 校验不通过的情况下, 打印Header, 方便查日志
				},
				"req.param": req_param,
				"req.opts":  req_opts,
			}).Warn(err)
			responseError(w, header, code, err.Error(), st)
			return err
		}
	}

	// 根据参数类型获取kvMap
	kvMap, err := getKVMap(r, req_param.ParamType)
	if err != nil {
		header := http.StatusInternalServerError
		code := int32(GateWayProtos.ResultType_ERR_Decode_Request)
		logger.Log().WithFields(logger.Fields{
			"http.Request": logger.Fields{
				"ClientIP": client_ip,
				"Method":   r.Method,
				"Host":     r.Host,
				"URL":      r.URL.String(),
			},
			"req.param": req_param,
			"req.opts":  req_opts,
		}).Error(err)
		responseError(w, header, code, err.Error(), st)
		return err
	}

	// kvMap 转换为 proto 转换为 []byte
	// P.s> 如果 RequestProto 为nil, 说明没有请求Proto
	var request []byte
	if req_opts.RequestProto != nil {
		request, err = kvMap2pb(kvMap, req_opts.RequestProto)
		if err != nil {
			header := http.StatusInternalServerError
			code := int32(GateWayProtos.ResultType_ERR_Decode_Request)
			logger.Log().WithFields(logger.Fields{
				"http.Request": logger.Fields{
					"ClientIP": client_ip,
					"Method":   r.Method,
					"Host":     r.Host,
					"URL":      r.URL.String(),
				},
				"req.param": req_param,
				"req.opts":  req_opts,
			}).Error(err)
			responseError(w, header, code, err.Error(), st)
			return err
		}
	}

	// 设置超时时间, 如果设置为0标识不超时
	var ctx context.Context
	var cancel context.CancelFunc
	if req_opts.Timeout == 0 {
		ctx, cancel = context.WithCancel(r.Context())
	} else {
		timeout := time.Duration(req_opts.Timeout) * time.Millisecond
		ctx, cancel = context.WithTimeout(r.Context(), timeout)
	}
	defer cancel()

	// 设置负载均衡方案
	if req_opts.LBPolicy == LBPolicy_ConsistentHash ||
		req_opts.LBPolicy == LBPolicy_SpecifyAddr {
		if req_opts.getLBKeyFunc == nil {
			header := http.StatusInternalServerError
			code := int32(GateWayProtos.ResultType_ERR_Decode_Request)
			msg := "get load balancer key failed"
			logger.Log().WithFields(logger.Fields{
				"http.Request": logger.Fields{
					"ClientIP": client_ip,
					"Method":   r.Method,
					"Host":     r.Host,
					"URL":      r.URL.String(),
				},
				"req.param": req_param,
				"req.opts":  req_opts,
			}).Warn(msg)
			responseError(w, header, code, msg, st)
			return errors.New(msg)
		}

		data := make(map[string]string)
		data[RegisterCenter.Param_PickType] = string(req_opts.LBPolicy)
		data[RegisterCenter.Param_PickParam] = req_opts.getLBKeyFunc()
		ctx = RegisterCenter.BuildCtxFilter(ctx, data)
	}

	// 发送请求
	var response []byte
	var result int32
	response, result, err = httpMsg.RegCenter.CallService(
		ctx, req_param.ServiceType, req_param.CMD, request)

	// 判断返回错误
	if err != nil {
		// 启用兜底返回
		if req_opts.GroundRules && req_opts.groundRulesFunc != nil {
			if ground_err := req_opts.groundRulesFunc(); ground_err == nil {
				// 统计兜底返回的日志
				logger.Log().WithFields(logger.Fields{
					"http.Request": logger.Fields{
						"ClientIP": client_ip,
						"Method":   r.Method,
						"Host":     r.Host,
						"URL":      r.URL.String(),
					},
					"req.param":    req_param,
					"req.opts":     req_opts,
					"ground_rules": "succ",
				}).Warn(err)
				return nil
			}
		}

		header := http.StatusInternalServerError
		code := result
		logger.Log().WithFields(logger.Fields{
			"http.Request": logger.Fields{
				"ClientIP": client_ip,
				"Method":   r.Method,
				"Host":     r.Host,
				"URL":      r.URL.String(),
			},
			"req.param": req_param,
			"req.opts":  req_opts,
		}).Error(err)
		responseError(w, header, code, err.Error(), st)
		return err
	}

	// 判断返回结果
	if result != int32(GateWayProtos.ResultType_OK) {
		// 启用兜底返回
		err = errors.New(string(response))
		if req_opts.GroundRules && req_opts.groundRulesFunc != nil {
			if ground_err := req_opts.groundRulesFunc(); ground_err == nil {
				// 统计兜底返回的日志
				logger.Log().WithFields(logger.Fields{
					"http.Request": logger.Fields{
						"ClientIP": client_ip,
						"Method":   r.Method,
						"Host":     r.Host,
						"URL":      r.URL.String(),
					},
					"req.param":    req_param,
					"req.opts":     req_opts,
					"ground_rules": "succ",
				}).Warn(err)
				return nil
			}
		}

		header := http.StatusOK
		code := result
		logger.Log().WithFields(logger.Fields{
			"http.Request": logger.Fields{
				"ClientIP": client_ip,
				"Method":   r.Method,
				"Host":     r.Host,
				"URL":      r.URL.String(),
			},
			"req.param": req_param,
			"req.opts":  req_opts,
			"code":      result,
		}).Error(err)
		responseError(w, header, code, err.Error(), st)
		return err
	}

	// 如果返回Proto为nil, 则说明下级服务采用Json格式返回
	if req_opts.ResponseProto == nil {
		logger.Log().WithFields(logger.Fields{
			"http.Request": logger.Fields{
				"ClientIP": client_ip,
				"Method":   r.Method,
				"Host":     r.Host,
				"URL":      r.URL.String(),
			},
			"req.param": req_param,
			"req.opts":  req_opts,
			"code":      result,
			"data":      response, // 将Response直接作为Json返回
		}).Debug("ok")
		responseJson(w, http.StatusOK, result, "ok", st, json.RawMessage(response))
		return nil
	}

	if string(response) != "ok" { // 针对房间服返回的补丁
		// []byte 转 proto
		if err := protoV2.Unmarshal(response, req_opts.ResponseProto); err != nil {
			code := int32(GateWayProtos.ResultType_ERR_Decode_Response)
			msg := "ResponseProto Unmarshal Error"
			logger.Log().WithFields(logger.Fields{
				"http.Request": logger.Fields{
					"ClientIP": client_ip,
					"Method":   r.Method,
					"Host":     r.Host,
					"URL":      r.URL.String(),
				},
				"req.param": req_param,
				"req.opts":  req_opts,
				"code":      code,
				"err":       err,
				// "response":  string(response),
			}).Error(msg)
			responseError(w, http.StatusOK, code, msg, st)
			return err
		}
	}

	logger.Log().WithFields(logger.Fields{
		"http.Request": logger.Fields{
			"ClientIP": client_ip,
			"Method":   r.Method,
			"Host":     r.Host,
			"URL":      r.URL.String(),
		},
		"req.param": req_param,
		"req.opts":  req_opts,
		"code":      result,
	}).Debug("ok")

	// 返回结果
	responseProto(w, http.StatusOK, result, "ok", st, req_opts.ResponseProto)
	return nil
}
