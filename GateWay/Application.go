package main

import (
	"AlgoGateWay/GateWay/HTTPMessage"
	"GateWayCommon/GateWayProtos"
	"GateWayCommon/RegisterCenter"
	"GateWayCommon/logger"
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Application struct {
	localIP      string
	listenPort   string
	Conf         *Config
	GrpcReceiver *GrpcMessage
	HttpReceiver *HTTPMessage.HttpMessage
	RegCenter    *RegisterCenter.RegisterCenter
}

var application *Application
var applicationOnce sync.Once

func GetApplication() *Application {
	applicationOnce.Do(func() {
		application = new(Application)
	})
	return application
}

// 初始化
func (app *Application) Init(
	localIP string,
	listenPort string,
	srvExe string,
	nickname string,
	configPath string,
) bool {
	app.localIP = localIP
	if app.localIP == "" {
		logger.Log().Error("input localIP error")
		return false
	}

	app.listenPort = listenPort
	if app.listenPort == "" {
		logger.Log().Error("input listenPort error")
		return false
	}

	app.Conf = new(Config)
	if app.Conf.Init(configPath) == false {
		logger.Log().Error("Config Init error")
		return false
	}

	app.GrpcReceiver = new(GrpcMessage)
	if app.GrpcReceiver.Init() == false {
		logger.Log().Error("GrpcReceiver Init error")
		return false
	}

	// 判断注册中心地址的长度大于0
	if len(app.Conf.g_config.RegisterCenterAddr) == 0 {
		logger.Log().Error("Config RegisterCenterAddr Size error")
		return false
	}

	hostname, _ := os.Hostname()
	localAddr := app.localIP + ":" + app.listenPort
	regAddrList := app.Conf.g_config.RegisterCenterAddr
	serviceInfo := &GateWayProtos.ServiceInfo{
		ServiceType:   int32(GateWayProtos.ServiceType_SERVICE_ALGO_GATE_WAY),
		Semver:        GateWayVersion,
		Addr:          localAddr,
		HostName:      hostname,
		Status:        int32(GateWayProtos.ServiceStatus_Online),
		ServiceWeight: 32,
		ConnectMode:   int32(GateWayProtos.ConnectMode_GRPC),
		GroupTab:      app.Conf.g_config.ServiceGroupTab,
		ServiceName:   srvExe,
		Nickname:      nickname,
	}

	serviceInfo.RelyList = append(serviceInfo.RelyList, &GateWayProtos.RelyInfo{
		RelyServiceType: int32(GateWayProtos.ServiceType_SERVICE_ALGO_CENTER),
		RelySemver:      AlgoCenterVersion,
	})

	app.RegCenter = new(RegisterCenter.RegisterCenter)
	if err := app.RegCenter.Init(serviceInfo, regAddrList); err != nil {
		logger.Log().WithFields(logger.Fields{
			"version":   GateWayVersion,
			"localAddr": localAddr,
			"regAddr":   regAddrList,
			"err":       err,
		}).Error("RegisterCenter Init error")
		return false
	}

	time.Sleep(1)

	// 将HttpMessage注册后移, 放到注册中心之后
	app.HttpReceiver = new(HTTPMessage.HttpMessage)
	if app.HttpReceiver.Init(localAddr, app.RegCenter) == false {
		logger.Log().Error("HttpReceiver Init error")
		return false
	}

	logger.Log().Info("Application Init Succ")
	return true
}

// 关闭
func (app *Application) Close() {
	logger.Log().Info("Application Close")
}

// 监听端口
func (app *Application) Start() {
	var listenAddr = app.Conf.GetConfig().IP + ":" + app.listenPort

	// 这里的实现参考了http.ListenAndServe(), 具体方案:
	// 1. 先Listen, 检查端口号是否可用
	// 2. 注册注册中心, 获得下级服务
	// 3. 最后Serve, 处理服务
	// P.s> 如果不这样做, 要么注册中心初始化时间过晚, 要么注册中心初始化时间过早
	// a. 只有Listen成功, 确定地址可用, 才应该初始化注册中心
	//      若先向注册中心发送了注册信息, 但因为Listen地址不可用退出(比如进程重复启动), 又会发送下线信息, 影响现有服务
	// b. 如果开始serve以后才初始化注册中心, 则有些晚了.
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Log().WithFields(logger.Fields{
			"addr": listenAddr,
			"err":  err,
		}).Error("net.Listen Error")
		return
	}
	logger.Log().WithField("addr", listenAddr).Info("net.Listen Succ")

	// 服务上线
	if err := app.RegCenter.Online(); err != nil {
		logger.Log().WithField("err", err).Info("RegisterCenter Online Failed")
		ln.Close()
		return
	}

	// http服务
	s := &http.Server{
		Addr:         listenAddr,
		Handler:      app.HandlerFunc(),
		ReadTimeout:  time.Duration(app.Conf.GetConfig().Http.ReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(app.Conf.GetConfig().Http.WriteTimeout) * time.Millisecond,
	}

	// 监听退出消息
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)

	// 协程处理服务
	go func() {
		logger.Log().Info("Serving...")
		if err := s.Serve(ln); err != nil {
			if err == http.ErrServerClosed {
				logger.Log().Info("Http Server Closed")
			} else {
				logger.Log().WithField("err", err).Error("Http Serve Error")
			}
			close(quit)
		}
	}()

	// 等待退出消息
	<-quit

	logger.Log().Info("Application Exiting...")

	// 注册中心客户端下线
	app.RegCenter.Offline()

	// 设定1秒超时, 停止Http服务
	logger.Log().Info("Shutdown Http Server...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		logger.Log().WithField("err", err).Info("Shutdown Http Server Error")
	}

	// 停止grpc服务
	app.GrpcReceiver.Stop()
}

// grpc/http 消息分发
func (app *Application) HandlerFunc() http.Handler {
	return h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			app.GrpcReceiver.Handler(w, r)
		} else {
			app.HttpReceiver.Handler(w, r)
		}
	}), &http2.Server{})
}
