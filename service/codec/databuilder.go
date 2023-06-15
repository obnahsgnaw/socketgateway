package codec

import (
	"encoding/json"
	"errors"
	"google.golang.org/protobuf/proto"
)

var (
	ErrNotAProtobufMessage = errors.New("Data builder pack failed, not a proto.Message ")
)

type DataPtr interface{}

// DataBuilder 包构建器
type DataBuilder interface {
	// Unpack  b 数据 p 对象指针
	Unpack(b []byte, p DataPtr) (err error)
	// Pack p 对象指针
	Pack(p DataPtr) (b []byte, err error)
}

type protobufDataBuilder struct {
}

func NewProtobufDataBuilder() DataBuilder {
	return &protobufDataBuilder{}
}

func (pb *protobufDataBuilder) Unpack(b []byte, p DataPtr) (err error) {
	if len(b) == 0 {
		return
	}
	if m, ok := p.(proto.Message); ok {
		err = proto.Unmarshal(b, m)
	} else {
		err = ErrNotAProtobufMessage
	}

	return
}
func (pb *protobufDataBuilder) Pack(p DataPtr) (b []byte, err error) {
	if p == nil {
		return
	}
	if m, ok := p.(proto.Message); ok {
		b, err = proto.Marshal(m)
	} else {
		err = ErrNotAProtobufMessage
	}

	return
}

type jsonDataBuilder struct {
}

func NewJsonDataBuilder() DataBuilder {
	return &jsonDataBuilder{}
}

func (pb *jsonDataBuilder) Unpack(b []byte, p DataPtr) (err error) {
	if len(b) == 0 {
		return
	}
	return json.Unmarshal(b, p)
}
func (pb *jsonDataBuilder) Pack(p DataPtr) (b []byte, err error) {
	if p == nil {
		return
	}
	return json.Marshal(p)
}
