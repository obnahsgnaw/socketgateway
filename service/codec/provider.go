package codec

import "sync"

type Provider func(firstPkg []byte) (Name, Codec, PkgBuilder)

type Name string

func (n Name) String() string {
	return string(n)
}

const (
	Proto Name = "proto"
	Json  Name = "json"
)

func DefaultProvider() Provider {
	return func(firstPkg []byte) (Name, Codec, PkgBuilder) {
		if firstPkg[0] == byte('{') {
			return Json, NewDelimiterCodec([]byte("SWOOLE_SOCKET_FINISH"), []byte("SWOOLEFN")), NewJsonPackageBuilder()
		}

		return Proto, NewLengthCodec(0xAB, 1024), NewProtobufPackageBuilder()
	}
}
func WssProvider() Provider {
	return func(firstPkg []byte) (Name, Codec, PkgBuilder) {
		if firstPkg[0] == byte('{') {
			return Json, NewWebsocketCodec(), NewJsonPackageBuilder()
		}

		return Proto, NewWebsocketCodec(), NewProtobufPackageBuilder()
	}
}

type DataBuilderProvider interface {
	Provider(Name) DataBuilder
}

type Dbp struct {
	json  DataBuilder
	proto DataBuilder
	sync.Once
}

func NewDbp() *Dbp {
	return &Dbp{}
}

func (p *Dbp) Provider(name Name) DataBuilder {
	if name == Json {
		p.Do(func() {
			p.json = NewJsonDataBuilder()
		})
		return p.json
	}

	p.Do(func() {
		p.proto = NewProtobufDataBuilder()
	})
	return p.proto
}
