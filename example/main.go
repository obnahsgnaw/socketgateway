package main

import (
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/http"
	"github.com/obnahsgnaw/http/engine"
	rpc2 "github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/socketgateway"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/gnet"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/moc"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"strconv"
)

func main() {
	app := application.New(
		"demo",
		application.Debug(func() bool {
			return true
		}),
		application.Logger(&logger.Config{
			Dir:        "",
			MaxSize:    5,
			MaxBackup:  1,
			MaxAge:     1,
			Level:      "debug",
			TraceLevel: "error",
		}),
	)
	defer app.Release()

	s := socketgateway.New(app, sockettype.TCP, endtype.Frontend, url.Host{Ip: "127.0.0.1", Port: 8001})
	s.With(socketgateway.Engine(gnet.New()))
	l, _ := rpc2.NewListener(url.Host{Ip: "127.0.0.1", Port: 8002})
	rps := rpc2.New(app, l, "gw", "gw", endtype.Frontend, nil)
	s.With(socketgateway.Rpc(rps))
	e, _ := http.Default("127.0.0.1", 8003, &engine.Config{
		Name:      "gw",
		DebugMode: false,
		Cors:      nil,
	})
	s.With(socketgateway.Doc(e))
	s.With(socketgateway.Watcher(func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field) {
		s.Logger().Debug(strconv.Itoa(c.Fd()) + ": " + msg)
	}))
	s.With(socketgateway.ReuseAddr())
	s.With(socketgateway.DefaultUser(&socket.AuthUser{
		Id:   0,
		Name: "system",
		Attr: nil,
	}))
	s.With(socketgateway.SecEncode(true))
	s.With(socketgateway.SecPrivateKey([]byte(`-----BEGIN rsa private key-----
MIICXQIBAAKBgQDbtIIwouJPHVuzSUYRYkhZ24SNcZTiyMkJ4WGdDsOI3OVuKlbX
jetM9VR4kRNZEyebJlbsW3TBF+6KO8U7jadLS1pVpULvpmrCHBhx5g+m6TVxlmMA
C9t4AB21H1qGaB65GGsS911EkA651m10GHFHtbFKqjQk0OgtTx95ETLv3wIDAQAB
AoGABfowPJCB5bMfxo3syRZKb59oSMzZRx49UfZ+yF4ZdcTEvS2LtUuYJjzacnlH
9Hlv72monb+TOpmjFMGxiQA+l+HpYis0/YAXYHwdilTOLOr6nqsQVr5pwye8RrzX
pRfjaJ95pexlvwfXwfX+NNzmEVvFLKbBpAmtdjL1IoKdp6ECQQDzEboKXxvOwAGL
MUgn4orq4iZCxB5UeKmJk/CfPhy4xRW8IYRl0MiZ5ojQmPZJr9ptgRV7cRSbUtQu
OFZYl+jRAkEA52SYEwrHe6amOHSGA5hm60u8OX0GXUosSq9RMKhEa/Cb6T4Db0fv
md3YnUfy888Hpt+ZjcTQ2HkUuZ9Hg1B5rwJBAJExForZcvfV01Y8stg4Rzi0q5wY
H/HfPY4Tk5jbdjacazY8YySaSSk8/p5zsDIl2/irMZTR4DhDisCtIE69NvECQQCM
044yWKcbvEsBpPlDCufoYEmH+216LYBTW+vf3yj1QJTDGXjhqzhJvtjssDNO6ztO
9lrwC07H0LkqV6QgaUQFAkAvLC2U4HYc7As/NPz48cNQz8opRGvIyG9g8sI+RZE6
UPVJ6NMli2MBL5Noj60dDNcNbKMS3D5yB2HxFf7PxEq+
-----END rsa private key-----
`)))

	// http 请求转发到tcp handler
	e.Engine().POST("/test", func(c *gin.Context) {
		// 初始化conn
		cc := moc.NewHttpConn(1, c.Request)
		var auth *socket.AuthUser
		if s.AuthEnabled() {
			// TODO 构建用户信息
		}
		s.Handler().ProxyConn(cc, auth, codec.Json, true)

		// 读取 请求数据
		pkg, _ := io.ReadAll(c.Request.Body)
		// 重新封包

		// 或者直接转发
		_, _, _, _, body, _ := s.Handler().ProxyMessage(cc, "123", pkg)
		if len(body) > 0 {
			c.String(200, string(body))
		}
	})

	app.AddServer(s)
	app.Run(func(err error) {
		panic(err)
	})
	app.Wait()
}
