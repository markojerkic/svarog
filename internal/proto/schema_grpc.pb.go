// Copyright 2015 gRPC authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.12.4
// source: internal/proto/schema.proto

package rpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	LoggAggregator_Log_FullMethodName      = "/svarog.LoggAggregator/Log"
	LoggAggregator_BatchLog_FullMethodName = "/svarog.LoggAggregator/BatchLog"
)

// LoggAggregatorClient is the client API for LoggAggregator service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type LoggAggregatorClient interface {
	Log(ctx context.Context, opts ...grpc.CallOption) (LoggAggregator_LogClient, error)
	BatchLog(ctx context.Context, in *Backlog, opts ...grpc.CallOption) (*Void, error)
}

type loggAggregatorClient struct {
	cc grpc.ClientConnInterface
}

func NewLoggAggregatorClient(cc grpc.ClientConnInterface) LoggAggregatorClient {
	return &loggAggregatorClient{cc}
}

func (c *loggAggregatorClient) Log(ctx context.Context, opts ...grpc.CallOption) (LoggAggregator_LogClient, error) {
	stream, err := c.cc.NewStream(ctx, &LoggAggregator_ServiceDesc.Streams[0], LoggAggregator_Log_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &loggAggregatorLogClient{stream}
	return x, nil
}

type LoggAggregator_LogClient interface {
	Send(*LogLine) error
	CloseAndRecv() (*Void, error)
	grpc.ClientStream
}

type loggAggregatorLogClient struct {
	grpc.ClientStream
}

func (x *loggAggregatorLogClient) Send(m *LogLine) error {
	return x.ClientStream.SendMsg(m)
}

func (x *loggAggregatorLogClient) CloseAndRecv() (*Void, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Void)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *loggAggregatorClient) BatchLog(ctx context.Context, in *Backlog, opts ...grpc.CallOption) (*Void, error) {
	out := new(Void)
	err := c.cc.Invoke(ctx, LoggAggregator_BatchLog_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LoggAggregatorServer is the server API for LoggAggregator service.
// All implementations must embed UnimplementedLoggAggregatorServer
// for forward compatibility
type LoggAggregatorServer interface {
	Log(LoggAggregator_LogServer) error
	BatchLog(context.Context, *Backlog) (*Void, error)
	mustEmbedUnimplementedLoggAggregatorServer()
}

// UnimplementedLoggAggregatorServer must be embedded to have forward compatible implementations.
type UnimplementedLoggAggregatorServer struct {
}

func (UnimplementedLoggAggregatorServer) Log(LoggAggregator_LogServer) error {
	return status.Errorf(codes.Unimplemented, "method Log not implemented")
}
func (UnimplementedLoggAggregatorServer) BatchLog(context.Context, *Backlog) (*Void, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BatchLog not implemented")
}
func (UnimplementedLoggAggregatorServer) mustEmbedUnimplementedLoggAggregatorServer() {}

// UnsafeLoggAggregatorServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LoggAggregatorServer will
// result in compilation errors.
type UnsafeLoggAggregatorServer interface {
	mustEmbedUnimplementedLoggAggregatorServer()
}

func RegisterLoggAggregatorServer(s grpc.ServiceRegistrar, srv LoggAggregatorServer) {
	s.RegisterService(&LoggAggregator_ServiceDesc, srv)
}

func _LoggAggregator_Log_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(LoggAggregatorServer).Log(&loggAggregatorLogServer{stream})
}

type LoggAggregator_LogServer interface {
	SendAndClose(*Void) error
	Recv() (*LogLine, error)
	grpc.ServerStream
}

type loggAggregatorLogServer struct {
	grpc.ServerStream
}

func (x *loggAggregatorLogServer) SendAndClose(m *Void) error {
	return x.ServerStream.SendMsg(m)
}

func (x *loggAggregatorLogServer) Recv() (*LogLine, error) {
	m := new(LogLine)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _LoggAggregator_BatchLog_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Backlog)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LoggAggregatorServer).BatchLog(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LoggAggregator_BatchLog_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LoggAggregatorServer).BatchLog(ctx, req.(*Backlog))
	}
	return interceptor(ctx, in, info, handler)
}

// LoggAggregator_ServiceDesc is the grpc.ServiceDesc for LoggAggregator service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var LoggAggregator_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "svarog.LoggAggregator",
	HandlerType: (*LoggAggregatorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "BatchLog",
			Handler:    _LoggAggregator_BatchLog_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Log",
			Handler:       _LoggAggregator_Log_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "internal/proto/schema.proto",
}
