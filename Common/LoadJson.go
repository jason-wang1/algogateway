package GateWayCommon

import (
	"GateWayCommon/logger"
	"encoding/json"
	"io/ioutil"
)

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

func (jst *JsonStruct) Load(filename string, v interface{}) bool {
	//ReadFile函数会读取文件的全部内容，并将结果以[]byte类型返回
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		logger.Log().WithField("err", err).Error("json.ReadFile Error")
		return false
	}

	//读取的数据为json格式，需要进行解码
	err = json.Unmarshal(data, v)
	if err != nil {
		logger.Log().WithField("err", err).Error("json.Unmarshal Error")
		return false
	}
	return true
}
