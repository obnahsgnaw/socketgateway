package codec

import "sync"

type Provider func(firstPkg []byte) (Name, Codec, PkgBuilder, []byte)

type Name string

func (n Name) String() string {
	return string(n)
}

const (
	Proto Name = "proto"
	Json  Name = "json"
)

func DefaultProvider() Provider {
	return func(firstPkg []byte) (Name, Codec, PkgBuilder, []byte) {
		tag := firstPkg[0]
		firstPkg = firstPkg[1:]
		if tag == byte('j') {
			return Json, NewDelimiterCodec([]byte("SWOOLE_SOCKET_FINISH"), []byte("SWOOLEFN")), NewJsonPackageBuilder(), firstPkg
		}

		return Proto, NewLengthCodec(0xAB, 1024), NewProtobufPackageBuilder(), firstPkg
	}
}
func WssProvider() Provider {
	return func(firstPkg []byte) (Name, Codec, PkgBuilder, []byte) {
		tag := firstPkg[0]
		firstPkg = firstPkg[1:]
		if tag == byte('j') {
			return Json, NewWebsocketCodec(), NewJsonPackageBuilder(), firstPkg
		}

		return Proto, NewWebsocketCodec(), NewProtobufPackageBuilder(), firstPkg
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
