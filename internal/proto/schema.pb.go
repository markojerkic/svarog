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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v3.12.4
// source: proto/schema.proto

package rpc

import (
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type LogLevel int32

const (
	LogLevel_DEBUG   LogLevel = 0
	LogLevel_INFO    LogLevel = 1
	LogLevel_WARNING LogLevel = 2
	LogLevel_ERROR   LogLevel = 3
)

// Enum value maps for LogLevel.
var (
	LogLevel_name = map[int32]string{
		0: "DEBUG",
		1: "INFO",
		2: "WARNING",
		3: "ERROR",
	}
	LogLevel_value = map[string]int32{
		"DEBUG":   0,
		"INFO":    1,
		"WARNING": 2,
		"ERROR":   3,
	}
)

func (x LogLevel) Enum() *LogLevel {
	p := new(LogLevel)
	*p = x
	return p
}

func (x LogLevel) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogLevel) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_schema_proto_enumTypes[0].Descriptor()
}

func (LogLevel) Type() protoreflect.EnumType {
	return &file_proto_schema_proto_enumTypes[0]
}

func (x LogLevel) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogLevel.Descriptor instead.
func (LogLevel) EnumDescriptor() ([]byte, []int) {
	return file_proto_schema_proto_rawDescGZIP(), []int{0}
}

type Void struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Void) Reset() {
	*x = Void{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_schema_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Void) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Void) ProtoMessage() {}

func (x *Void) ProtoReflect() protoreflect.Message {
	mi := &file_proto_schema_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Void.ProtoReflect.Descriptor instead.
func (*Void) Descriptor() ([]byte, []int) {
	return file_proto_schema_proto_rawDescGZIP(), []int{0}
}

type LogLine struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Client    string               `protobuf:"bytes,1,opt,name=client,proto3" json:"client,omitempty"`
	Message   string               `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Level     LogLevel             `protobuf:"varint,3,opt,name=level,proto3,enum=svarog.LogLevel" json:"level,omitempty"`
	Timestamp *timestamp.Timestamp `protobuf:"bytes,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Sequence  int64                `protobuf:"varint,5,opt,name=sequence,proto3" json:"sequence,omitempty"`
}

func (x *LogLine) Reset() {
	*x = LogLine{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_schema_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LogLine) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogLine) ProtoMessage() {}

func (x *LogLine) ProtoReflect() protoreflect.Message {
	mi := &file_proto_schema_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogLine.ProtoReflect.Descriptor instead.
func (*LogLine) Descriptor() ([]byte, []int) {
	return file_proto_schema_proto_rawDescGZIP(), []int{1}
}

func (x *LogLine) GetClient() string {
	if x != nil {
		return x.Client
	}
	return ""
}

func (x *LogLine) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *LogLine) GetLevel() LogLevel {
	if x != nil {
		return x.Level
	}
	return LogLevel_DEBUG
}

func (x *LogLine) GetTimestamp() *timestamp.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

func (x *LogLine) GetSequence() int64 {
	if x != nil {
		return x.Sequence
	}
	return 0
}

type Backlog struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Logs []*LogLine `protobuf:"bytes,1,rep,name=logs,proto3" json:"logs,omitempty"`
}

func (x *Backlog) Reset() {
	*x = Backlog{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_schema_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Backlog) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Backlog) ProtoMessage() {}

func (x *Backlog) ProtoReflect() protoreflect.Message {
	mi := &file_proto_schema_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Backlog.ProtoReflect.Descriptor instead.
func (*Backlog) Descriptor() ([]byte, []int) {
	return file_proto_schema_proto_rawDescGZIP(), []int{2}
}

func (x *Backlog) GetLogs() []*LogLine {
	if x != nil {
		return x.Logs
	}
	return nil
}

var File_proto_schema_proto protoreflect.FileDescriptor

var file_proto_schema_proto_rawDesc = []byte{
	0x0a, 0x12, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x73, 0x76, 0x61, 0x72, 0x6f, 0x67, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x06, 0x0a,
	0x04, 0x56, 0x6f, 0x69, 0x64, 0x22, 0xb9, 0x01, 0x0a, 0x07, 0x4c, 0x6f, 0x67, 0x4c, 0x69, 0x6e,
	0x65, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x12, 0x26, 0x0a, 0x05, 0x6c, 0x65, 0x76, 0x65, 0x6c, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x10, 0x2e, 0x73, 0x76, 0x61, 0x72, 0x6f, 0x67, 0x2e, 0x4c, 0x6f, 0x67, 0x4c,
	0x65, 0x76, 0x65, 0x6c, 0x52, 0x05, 0x6c, 0x65, 0x76, 0x65, 0x6c, 0x12, 0x38, 0x0a, 0x09, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x1a, 0x0a, 0x08, 0x73, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63,
	0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x73, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63,
	0x65, 0x22, 0x2e, 0x0a, 0x07, 0x42, 0x61, 0x63, 0x6b, 0x6c, 0x6f, 0x67, 0x12, 0x23, 0x0a, 0x04,
	0x6c, 0x6f, 0x67, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x73, 0x76, 0x61,
	0x72, 0x6f, 0x67, 0x2e, 0x4c, 0x6f, 0x67, 0x4c, 0x69, 0x6e, 0x65, 0x52, 0x04, 0x6c, 0x6f, 0x67,
	0x73, 0x2a, 0x37, 0x0a, 0x08, 0x4c, 0x6f, 0x67, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x12, 0x09, 0x0a,
	0x05, 0x44, 0x45, 0x42, 0x55, 0x47, 0x10, 0x00, 0x12, 0x08, 0x0a, 0x04, 0x49, 0x4e, 0x46, 0x4f,
	0x10, 0x01, 0x12, 0x0b, 0x0a, 0x07, 0x57, 0x41, 0x52, 0x4e, 0x49, 0x4e, 0x47, 0x10, 0x02, 0x12,
	0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x03, 0x32, 0x67, 0x0a, 0x0e, 0x4c, 0x6f,
	0x67, 0x67, 0x41, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x6f, 0x72, 0x12, 0x28, 0x0a, 0x03,
	0x4c, 0x6f, 0x67, 0x12, 0x0f, 0x2e, 0x73, 0x76, 0x61, 0x72, 0x6f, 0x67, 0x2e, 0x4c, 0x6f, 0x67,
	0x4c, 0x69, 0x6e, 0x65, 0x1a, 0x0c, 0x2e, 0x73, 0x76, 0x61, 0x72, 0x6f, 0x67, 0x2e, 0x56, 0x6f,
	0x69, 0x64, 0x22, 0x00, 0x28, 0x01, 0x12, 0x2b, 0x0a, 0x08, 0x42, 0x61, 0x74, 0x63, 0x68, 0x4c,
	0x6f, 0x67, 0x12, 0x0f, 0x2e, 0x73, 0x76, 0x61, 0x72, 0x6f, 0x67, 0x2e, 0x42, 0x61, 0x63, 0x6b,
	0x6c, 0x6f, 0x67, 0x1a, 0x0c, 0x2e, 0x73, 0x76, 0x61, 0x72, 0x6f, 0x67, 0x2e, 0x56, 0x6f, 0x69,
	0x64, 0x22, 0x00, 0x42, 0x23, 0x5a, 0x21, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x6d, 0x61, 0x72, 0x6b, 0x6f, 0x6a, 0x65, 0x72, 0x6b, 0x69, 0x63, 0x2f, 0x73, 0x76,
	0x61, 0x72, 0x6f, 0x67, 0x2f, 0x72, 0x70, 0x63, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_schema_proto_rawDescOnce sync.Once
	file_proto_schema_proto_rawDescData = file_proto_schema_proto_rawDesc
)

func file_proto_schema_proto_rawDescGZIP() []byte {
	file_proto_schema_proto_rawDescOnce.Do(func() {
		file_proto_schema_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_schema_proto_rawDescData)
	})
	return file_proto_schema_proto_rawDescData
}

var file_proto_schema_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto_schema_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_proto_schema_proto_goTypes = []interface{}{
	(LogLevel)(0),               // 0: svarog.LogLevel
	(*Void)(nil),                // 1: svarog.Void
	(*LogLine)(nil),             // 2: svarog.LogLine
	(*Backlog)(nil),             // 3: svarog.Backlog
	(*timestamp.Timestamp)(nil), // 4: google.protobuf.Timestamp
}
var file_proto_schema_proto_depIdxs = []int32{
	0, // 0: svarog.LogLine.level:type_name -> svarog.LogLevel
	4, // 1: svarog.LogLine.timestamp:type_name -> google.protobuf.Timestamp
	2, // 2: svarog.Backlog.logs:type_name -> svarog.LogLine
	2, // 3: svarog.LoggAggregator.Log:input_type -> svarog.LogLine
	3, // 4: svarog.LoggAggregator.BatchLog:input_type -> svarog.Backlog
	1, // 5: svarog.LoggAggregator.Log:output_type -> svarog.Void
	1, // 6: svarog.LoggAggregator.BatchLog:output_type -> svarog.Void
	5, // [5:7] is the sub-list for method output_type
	3, // [3:5] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_proto_schema_proto_init() }
func file_proto_schema_proto_init() {
	if File_proto_schema_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_schema_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Void); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_schema_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LogLine); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_schema_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Backlog); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_schema_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_schema_proto_goTypes,
		DependencyIndexes: file_proto_schema_proto_depIdxs,
		EnumInfos:         file_proto_schema_proto_enumTypes,
		MessageInfos:      file_proto_schema_proto_msgTypes,
	}.Build()
	File_proto_schema_proto = out.File
	file_proto_schema_proto_rawDesc = nil
	file_proto_schema_proto_goTypes = nil
	file_proto_schema_proto_depIdxs = nil
}