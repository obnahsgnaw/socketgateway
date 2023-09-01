package codec

import (
	"encoding/json"
	"errors"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"google.golang.org/protobuf/proto"
	"strconv"
)

//
//import (
//	"encoding/json"
//	"errors"
//	"google.golang.org/protobuf/proto"
//)

type ActionId uint32

func (a ActionId) String() string {
	return strconv.Itoa(int(a))
}

// Action id
type Action struct {
	Id   ActionId
	Name string
}

func (a Action) String() string {
	return a.Id.String() + ":" + a.Name
}

func NewAction(id ActionId, name string) Action {
	return Action{
		Id:   id,
		Name: name,
	}
}

// Pkg 包结构
type Pkg struct {
	ActionId ActionId
	Package  []byte
	Iv       string
}

// PkgBuilder 包构建器
type PkgBuilder interface {
	Unpack(b []byte) (p *gatewayv1.GatewayPackage, err error)
	Pack(p *gatewayv1.GatewayPackage) (b []byte, err error)
}

func packErr(err error) error {
	return errors.New("pkg builder error: Pack package failed, err=" + err.Error())
}

func unpackErr(err error) error {
	return errors.New("pkg builder error: Unpack package failed, err=" + err.Error())
}

var (
	ErrNoData  = errors.New("pkg builder error: Unpack package failed, err= no data. ")
	ErrDataNil = errors.New("pkg builder error: pack package failed, err= data is nil. ")
)

// ProtobufPackageBuilder protobuf 包构建器
type ProtobufPackageBuilder struct{}

// NewProtobufPackageBuilder return a protobuf package builder
func NewProtobufPackageBuilder() *ProtobufPackageBuilder {
	return &ProtobufPackageBuilder{}
}

// Unpack 拆包
func (pp *ProtobufPackageBuilder) Unpack(b []byte) (p *gatewayv1.GatewayPackage, err error) {
	if len(b) == 0 {
		err = ErrNoData
		return
	}
	p1 := gatewayv1.GatewayPackage{}
	if err = proto.Unmarshal(b, &p1); err != nil {
		err = unpackErr(err)
	}
	p = &p1

	return
}

// Pack 封包
func (pp *ProtobufPackageBuilder) Pack(p *gatewayv1.GatewayPackage) (b []byte, err error) {
	if p == nil {
		err = ErrDataNil
		return
	}
	if b, err = proto.Marshal(p); err != nil {
		err = packErr(err)
	}

	return
}

type JsonPackageBuilder struct{}

func NewJsonPackageBuilder() *JsonPackageBuilder {
	return &JsonPackageBuilder{}
}

// Unpack 拆包
func (pp *JsonPackageBuilder) Unpack(b []byte) (p *gatewayv1.GatewayPackage, err error) {
	if len(b) == 0 {
		err = ErrNoData
		return
	}
	p1 := gatewayv1.GatewayPackage{}
	if err = json.Unmarshal(b, &p1); err != nil {
		err = unpackErr(err)
	}

	p = &p1

	return
}

// Pack 封包
func (pp *JsonPackageBuilder) Pack(p *gatewayv1.GatewayPackage) (b []byte, err error) {
	if p == nil {
		err = ErrDataNil
		return
	}
	if b, err = json.Marshal(p); err != nil {
		err = packErr(err)
	}

	return
}
