package dfuse

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var plainTextDialOption = grpc.WithInsecure()
var insecureTLSDialOption = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true}))
var secureTLSDialOption = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, ""))
var maxCallRecvMsgSize = 1024 * 1024 * 100
var defaultCallOptions = []grpc.CallOption{
	grpc.MaxCallRecvMsgSize(maxCallRecvMsgSize),
	grpc.WaitForReady(true),
}

var keepaliveDialOption = grpc.WithKeepaliveParams(keepalive.ClientParameters{
	// Send pings every (x seconds) there is no activity
	Time: 30 * time.Second,
	// Wait that amount of time for ping ack before considering the connection dead
	Timeout: 10 * time.Second,
	// Send pings even without active streams
	PermitWithoutStream: true,
})

func newGRPCClient(remoteAddr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	options := []grpc.DialOption{
		keepaliveDialOption,
		grpc.WithDefaultCallOptions(defaultCallOptions...),
	}
	options = append(options, opts...)

	return grpc.Dial(remoteAddr, options...)
}
