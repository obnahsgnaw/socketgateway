package codec

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// PkgHandler 包处理函数
type PkgHandler func(pkg []byte)

// Codec Tcp 拆包编解码
type Codec interface {
	// Marshal 编码 输入数据 返航编码后的数据 如果出现错误返航error
	Marshal(b []byte) (d []byte, err error)
	// Unmarshal 解码 输入数据 解码后执行handler返回剩下的包数据 出现错误返航error
	Unmarshal(b []byte, handler PkgHandler) (tmp []byte, err error)
}

// 分隔符编解码
type delimiterCodec struct {
	sendDelimiter    []byte
	receiveDelimiter []byte
}

// NewDelimiterCodec 分隔符编解码
func NewDelimiterCodec(sendDelimiter, receiveDelimiter []byte) Codec {
	return &delimiterCodec{
		sendDelimiter:    sendDelimiter,
		receiveDelimiter: receiveDelimiter,
	}
}

func (codec *delimiterCodec) Marshal(b []byte) (d []byte, err error) {
	if !bytes.HasSuffix(b, codec.sendDelimiter) {
		d = append(b, codec.sendDelimiter...)
	}

	return
}

func (codec *delimiterCodec) Unmarshal(b []byte, handler PkgHandler) (tmp []byte, err error) {
	if len(b) == 0 {
		return
	}

	idx := bytes.LastIndex(b, codec.receiveDelimiter)
	if idx <= 0 {
		tmp = b
		return
	}

	pkgs := b[:idx]
	tmp = b[idx+len(codec.receiveDelimiter):]

	if handler == nil {
		return
	}

	pkgs = bytes.TrimPrefix(pkgs, codec.receiveDelimiter)
	pkgs = bytes.TrimSuffix(pkgs, codec.receiveDelimiter)

	if len(pkgs) == 0 {
		return
	}

	if bytes.Index(pkgs, codec.receiveDelimiter) == -1 {
		handler(pkgs)
	} else {
		for _, p := range bytes.Split(pkgs, codec.receiveDelimiter) {
			if len(p) > 0 {
				handler(p)
			}
		}
	}

	return
}

// WebsocketCodec websocket的包编解码
type websocketCodec struct{}

// NewWebsocketCodec websocket的包编解码
func NewWebsocketCodec() Codec {
	return &websocketCodec{}
}

func (codec *websocketCodec) Marshal(b []byte) (d []byte, err error) {
	if len(b) == 0 {
		err = errors.New("codec error: Codec encode package failed, package is empty. ")
		return
	}
	d = b
	return
}

func (codec *websocketCodec) Unmarshal(b []byte, handler PkgHandler) (tmp []byte, err error) {
	if handler != nil {
		handler(b)
	}
	return
}

// LengthCodec Protocol format:
// length 2byte + pkg
type lengthCodec struct {
	magicNumber      uint16
	magicNumberSize  int
	magicNumberBytes []byte
	bodySize         int
	bodyMaxSize      int
}

// NewLengthCodec 包头设置包体长度的编解码
func NewLengthCodec(magicNumber uint16, bodyMax int) Codec {
	var magicNumberBytes []byte
	magicNumberSize := 0
	if magicNumber != 0 {
		magicNumberSize = 2
		magicNumberBytes = make([]byte, magicNumberSize)
		binary.BigEndian.PutUint16(magicNumberBytes, magicNumber)
	}
	return &lengthCodec{
		magicNumber:      magicNumber,
		magicNumberSize:  magicNumberSize,
		magicNumberBytes: magicNumberBytes,
		bodySize:         4,
		bodyMaxSize:      bodyMax,
	}
}

var (
	ErrPkgEmpty        = errors.New("codec error: Codec encode package failed, package is empty. ")
	ErrPkgTooLong      = errors.New("codec error: Codec encode package failed, package too long. ")
	ErrInvalidMagicNum = errors.New("codec error: Codec decode package failed, invalid magic number ")
)

func (codec *lengthCodec) Marshal(b []byte) (d []byte, err error) {
	// 验证长度
	if len(b) == 0 {
		err = ErrPkgEmpty
		return
	}
	if len(b) > codec.bodyMaxSize {
		err = ErrPkgTooLong
		return
	}
	bodyOffset := codec.magicNumberSize + codec.bodySize
	msgLen := bodyOffset + len(b)

	// 组装头验证数字
	d = make([]byte, msgLen)
	if codec.magicNumberSize > 0 {
		copy(d, codec.magicNumberBytes)
	}
	// 组装数据
	binary.BigEndian.PutUint32(d[codec.magicNumberSize:bodyOffset], uint32(len(b)))
	copy(d[bodyOffset:msgLen], b)

	return
}

func (codec *lengthCodec) Unmarshal(b []byte, handler PkgHandler) (tmp []byte, err error) {
	for {
		// 取头数据
		bodyOffset := codec.magicNumberSize + codec.bodySize

		if len(b) < bodyOffset {
			tmp = b
			return
		}
		// 比较头数据验证
		if codec.magicNumberSize > 0 && !bytes.Equal(codec.magicNumberBytes, b[:codec.magicNumberSize]) {
			tmp = b
			err = ErrInvalidMagicNum
			return
		}

		// 验证内容长度
		bodyLen := binary.BigEndian.Uint32(b[codec.magicNumberSize:bodyOffset])
		if bodyLen > uint32(codec.bodyMaxSize) {
			tmp = b
			err = ErrPkgTooLong
			return
		}
		msgLen := bodyOffset + int(bodyLen)
		if len(b) < msgLen {
			tmp = b
			return
		}

		rev := b[bodyOffset:msgLen]
		b = b[msgLen:]
		if handler != nil && len(rev) > 0 {
			handler(rev)
		}
		if len(b) == 0 {
			return
		}
	}
}
