// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: teleport/auditlog/v1/auditlog.proto

package auditlogv1

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
	AuditLogService_StreamUnstructuredSessionEvents_FullMethodName = "/teleport.auditlog.v1.AuditLogService/StreamUnstructuredSessionEvents"
	AuditLogService_GetUnstructuredEvents_FullMethodName           = "/teleport.auditlog.v1.AuditLogService/GetUnstructuredEvents"
)

// AuditLogServiceClient is the client API for AuditLogService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AuditLogServiceClient interface {
	// StreamUnstructuredSessionEvents streams audit events from a given session recording in an unstructured format.
	// This endpoint is used by the event handler to retrieve the session events as JSON.
	StreamUnstructuredSessionEvents(ctx context.Context, in *StreamUnstructuredSessionEventsRequest, opts ...grpc.CallOption) (AuditLogService_StreamUnstructuredSessionEventsClient, error)
	// GetUnstructuredEvents gets events from the audit log in an unstructured format.
	// This endpoint is used by the event handler to retrieve the events as JSON.
	GetUnstructuredEvents(ctx context.Context, in *GetUnstructuredEventsRequest, opts ...grpc.CallOption) (*EventsUnstructured, error)
}

type auditLogServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAuditLogServiceClient(cc grpc.ClientConnInterface) AuditLogServiceClient {
	return &auditLogServiceClient{cc}
}

func (c *auditLogServiceClient) StreamUnstructuredSessionEvents(ctx context.Context, in *StreamUnstructuredSessionEventsRequest, opts ...grpc.CallOption) (AuditLogService_StreamUnstructuredSessionEventsClient, error) {
	stream, err := c.cc.NewStream(ctx, &AuditLogService_ServiceDesc.Streams[0], AuditLogService_StreamUnstructuredSessionEvents_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &auditLogServiceStreamUnstructuredSessionEventsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type AuditLogService_StreamUnstructuredSessionEventsClient interface {
	Recv() (*EventUnstructured, error)
	grpc.ClientStream
}

type auditLogServiceStreamUnstructuredSessionEventsClient struct {
	grpc.ClientStream
}

func (x *auditLogServiceStreamUnstructuredSessionEventsClient) Recv() (*EventUnstructured, error) {
	m := new(EventUnstructured)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *auditLogServiceClient) GetUnstructuredEvents(ctx context.Context, in *GetUnstructuredEventsRequest, opts ...grpc.CallOption) (*EventsUnstructured, error) {
	out := new(EventsUnstructured)
	err := c.cc.Invoke(ctx, AuditLogService_GetUnstructuredEvents_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AuditLogServiceServer is the server API for AuditLogService service.
// All implementations must embed UnimplementedAuditLogServiceServer
// for forward compatibility
type AuditLogServiceServer interface {
	// StreamUnstructuredSessionEvents streams audit events from a given session recording in an unstructured format.
	// This endpoint is used by the event handler to retrieve the session events as JSON.
	StreamUnstructuredSessionEvents(*StreamUnstructuredSessionEventsRequest, AuditLogService_StreamUnstructuredSessionEventsServer) error
	// GetUnstructuredEvents gets events from the audit log in an unstructured format.
	// This endpoint is used by the event handler to retrieve the events as JSON.
	GetUnstructuredEvents(context.Context, *GetUnstructuredEventsRequest) (*EventsUnstructured, error)
	mustEmbedUnimplementedAuditLogServiceServer()
}

// UnimplementedAuditLogServiceServer must be embedded to have forward compatible implementations.
type UnimplementedAuditLogServiceServer struct {
}

func (UnimplementedAuditLogServiceServer) StreamUnstructuredSessionEvents(*StreamUnstructuredSessionEventsRequest, AuditLogService_StreamUnstructuredSessionEventsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamUnstructuredSessionEvents not implemented")
}
func (UnimplementedAuditLogServiceServer) GetUnstructuredEvents(context.Context, *GetUnstructuredEventsRequest) (*EventsUnstructured, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUnstructuredEvents not implemented")
}
func (UnimplementedAuditLogServiceServer) mustEmbedUnimplementedAuditLogServiceServer() {}

// UnsafeAuditLogServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AuditLogServiceServer will
// result in compilation errors.
type UnsafeAuditLogServiceServer interface {
	mustEmbedUnimplementedAuditLogServiceServer()
}

func RegisterAuditLogServiceServer(s grpc.ServiceRegistrar, srv AuditLogServiceServer) {
	s.RegisterService(&AuditLogService_ServiceDesc, srv)
}

func _AuditLogService_StreamUnstructuredSessionEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StreamUnstructuredSessionEventsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AuditLogServiceServer).StreamUnstructuredSessionEvents(m, &auditLogServiceStreamUnstructuredSessionEventsServer{stream})
}

type AuditLogService_StreamUnstructuredSessionEventsServer interface {
	Send(*EventUnstructured) error
	grpc.ServerStream
}

type auditLogServiceStreamUnstructuredSessionEventsServer struct {
	grpc.ServerStream
}

func (x *auditLogServiceStreamUnstructuredSessionEventsServer) Send(m *EventUnstructured) error {
	return x.ServerStream.SendMsg(m)
}

func _AuditLogService_GetUnstructuredEvents_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUnstructuredEventsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AuditLogServiceServer).GetUnstructuredEvents(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AuditLogService_GetUnstructuredEvents_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AuditLogServiceServer).GetUnstructuredEvents(ctx, req.(*GetUnstructuredEventsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// AuditLogService_ServiceDesc is the grpc.ServiceDesc for AuditLogService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var AuditLogService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "teleport.auditlog.v1.AuditLogService",
	HandlerType: (*AuditLogServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetUnstructuredEvents",
			Handler:    _AuditLogService_GetUnstructuredEvents_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamUnstructuredSessionEvents",
			Handler:       _AuditLogService_StreamUnstructuredSessionEvents_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "teleport/auditlog/v1/auditlog.proto",
}