package HTTPMessage

import (
	"GateWayCommon/GateWayProtos"
	"net/http"
)

const (
	api_v1_web_download   = "/api/v1/web/download/"
)

func (httpMsg *HttpMessage) init_download() {
	httpMsg.mux.HandleFunc(api_v1_web_download, httpMsg.api_v1_web_download)     // 下载推荐
}

// @Tags	下载推荐
// @Summary 下载
// @Router /api/v1/web/download/ [get]
// @Param request query GateWayProtos.AlgoCenterRequest true "请求Proto结构"
// @Produce json
// @Success 200 {object} jsonResponse{data=GateWayProtos.AlgoCenterResponse} "成功 code=0, msg=ok; 失败code=错误码, msg=错误信息."
func (httpMsg *HttpMessage) api_v1_web_download(
	w http.ResponseWriter,
	r *http.Request,
) {
	httpMsg.common_request_v3(
		w, r,
		&requestParam{
			FuncName:    get_func_name(),
			Method:      "GET",
			ParamType:   "query",
			ServiceType: int32(GateWayProtos.ServiceType_SERVICE_ALGO_CENTER),
			CMD:         int32(GateWayProtos.CmdType_CMD_GET_DOWNLOAD_RECOMMEND),
		},
		withTimeout(1000),
		withRequestProto(&GateWayProtos.AlgoCenterRequest{}),
		withResponseProto(&GateWayProtos.AlgoCenterResponse{}))
}
