package connutil

import (
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketutil/codec"
	"strconv"
)

func SetHeartbeatInterval(c socket.Conn, interval int64) {
	c.Context().SetOptional("heartbeat_interval", interval)
}

func GetHeartbeatInterval(c socket.Conn) int64 {
	if v, ok := c.Context().GetOptional("heartbeat_interval"); ok {
		return v.(int64)
	}
	return 0
}

func SetCloseReason(c socket.Conn, reason string) {
	c.Context().SetOptional("close_reason", reason)
}

func GetCloseReason(c socket.Conn) string {
	if v, ok := c.Context().GetOptional("close_reason"); ok {
		return v.(string)
	}
	return ""
}

func ProtoCoder(c socket.Conn) codec.Codec {
	v, _ := c.Context().GetOptional("codec")
	return v.(codec.Codec)
}

func GatewayPkgCoder(c socket.Conn) codec.PkgBuilder {
	v, _ := c.Context().GetOptional("pkgBuilder")
	return v.(codec.PkgBuilder)
}

func DataCoder(c socket.Conn) codec.DataBuilder {
	v, _ := c.Context().GetOptional("dataBuilder")
	return v.(codec.DataBuilder)
}

func CoderName(c socket.Conn) codec.Name {
	v, _ := c.Context().GetOptional("coderName")
	return v.(codec.Name)
}

func SetCoder(c socket.Conn, protoCoder codec.Codec, gatewayPkgCoder codec.PkgBuilder, dataCoder codec.DataBuilder) {
	c.Context().SetOptional("coderName", dataCoder.Name())
	c.Context().SetOptional("codec", protoCoder)
	c.Context().SetOptional("pkgBuilder", gatewayPkgCoder)
	c.Context().SetOptional("dataBuilder", dataCoder)
}

func ClearCoder(c socket.Conn) {
	c.Context().DelOptional("coderName")
	c.Context().DelOptional("codec")
	c.Context().DelOptional("pkgBuilder")
	c.Context().DelOptional("dataBuilder")
}

func CoderInitialized(c socket.Conn) bool {
	_, ok := c.Context().GetOptional("coderName")
	return ok
}

func SetCryptoKey(c socket.Conn, key []byte) {
	c.Context().SetOptional("cryptKey", key)
}

func CryptoKey(c socket.Conn) []byte {
	k, ok := c.Context().GetOptional("cryptKey")
	if !ok {
		return nil
	}
	return k.([]byte)
}

func ClearCryptoKey(c socket.Conn) {
	c.Context().DelOptional("cryptKey")
}

func WithTempPackager(c socket.Conn, pkg []byte, f func(pkg []byte) ([]byte, error)) (err error) {
	if v, ok := c.Context().GetAndDelOptional("pkgTmp"); ok {
		pkg = append(v.([]byte), pkg...)
	}
	var temp []byte
	if temp, err = f(pkg); err == nil {
		c.Context().SetOptional("pkgTmp", temp)
	}

	return
}

func GetActionFlb(c socket.Conn, actionId codec.ActionId) int {
	if v, ok := c.Context().GetOptional("flb-" + actionId.String()); ok {
		return v.(int)
	}
	return 0
}

func SetActionFlb(c socket.Conn, actionId, flb int) {
	c.Context().SetOptional("flb-"+strconv.Itoa(actionId), flb)
}
