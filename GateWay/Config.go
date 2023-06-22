package main

import "GateWayCommon"

type s_http struct {
	ReadTimeout  int
	WriteTimeout int
}

type s_swagger struct {
	Host        string
	BasePath    string
	Title       string
	Description string
}

type s_ground_rules struct {
	CMD      int32
	FileName string
}

type s_serverConfig struct {
	IP                 string
	Http               s_http
	RegisterCenterAddr []string
	ServiceGroupTab    string
}

type Config struct {
	g_config s_serverConfig
}

func (conf *Config) Init(filename string) bool {
	JsonParse := GateWayCommon.NewJsonStruct()
	return JsonParse.Load(filename, &conf.g_config)
}

func (conf *Config) GetConfig() s_serverConfig {
	return conf.g_config
}
