package main

import (
	"GateWayCommon/GateWayProtos"
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip" // Install the gzip compressor
	"google.golang.org/grpc/keepalive"
)

type GrpcMessage struct {
	grpcServer *grpc.Server
}

func (grpcMsg *GrpcMessage) Init() bool {
	var kaep = keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // If a client pings more than once every 5 seconds, terminate the connection
		PermitWithoutStream: true,            // Allow pings even when there are no active streams
	}

	var kasp = keepalive.ServerParameters{
		MaxConnectionIdle:     30 * time.Second, // If a client is idle for 30 seconds, send a GOAWAY
		MaxConnectionAgeGrace: 5 * time.Second,  // Allow 5 seconds for pending RPCs to complete before forcibly closing connections
		Time:                  10 * time.Second, // Ping the client if it is idle for 10 seconds to ensure the connection is still active
		Timeout:               1 * time.Second,  // Wait 1 second for the ping ack before assuming the connection is dead
	}

	grpcMsg.grpcServer = grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
	)

	GateWayProtos.RegisterUnifiedServiceServer(grpcMsg.grpcServer, grpcMsg)
	return true
}

func (grpcMsg *GrpcMessage) Stop() {
	// 优雅退出
	grpcMsg.grpcServer.GracefulStop()
}

func (grpcMsg *GrpcMessage) Handler(
	w http.ResponseWriter,
	r *http.Request,
) {
	grpcMsg.grpcServer.ServeHTTP(w, r)
}

func (grpcMsg *GrpcMessage) CallService(
	ctx context.Context,
	req *GateWayProtos.UnifiedRequest,
) (*GateWayProtos.UnifiedResponse, error) {
	// 消息派发
	cmd := req.GetCmd()
	if cmd == int32(GateWayProtos.CmdType_CMD_NOTIFY) {
		return GetApplication().RegCenter.OnNotify(ctx, req)
	} else if cmd == int32(GateWayProtos.CmdType_CMD_HELLO) {
		return GetApplication().RegCenter.OnHello(ctx, req)
	} else {
		resp := &GateWayProtos.UnifiedResponse{
			Cmd:      cmd,
			Result:   int32(GateWayProtos.ResultType_ERR_Service_CMD),
			Response: []byte("unsupported cmd."),
		}
		return resp, nil
	}
}
