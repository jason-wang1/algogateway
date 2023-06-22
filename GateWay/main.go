package main

import (
	"GateWayCommon/logger"
	"os"
)

func main() {
	if len(os.Args) < 5 {
		// 参数错误 ./exe ip port nickname name
		return
	}

	ip := os.Args[1]
	port := os.Args[2]
	nickname := os.Args[3]
	name := os.Args[4]

	// 日志模块初始化
	{
		log_dir := "../LogAlgoGateWay/"
		logger.Init(log_dir, name, logger.InfoLevel)
		logger.Log().WithFields(logger.Fields{
			"exe":     name,
			"version": GateWayVersion,
			"pid":     os.Getpid(),
		}).Info("application start...")
	}

	// 启动服务
	srvExe := "AlgoGateWay"
	configPath := "./config/AlgoGateWayConf.json"
	app := GetApplication()
	if app.Init(ip, port, srvExe, nickname, configPath) {
		app.Start()
	}
	app.Close()
}
