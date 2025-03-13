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
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/moc"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler/connutil"
	"github.com/obnahsgnaw/socketgateway/service/manage"
	"github.com/obnahsgnaw/socketutil/codec"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"log"
	"strconv"
	"time"
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
	s.With(socketgateway.Tick(time.Second * 30))
	s.With(socketgateway.Heartbeat(time.Second * 60))
	// http 请求转发到tcp handler
	e.Engine().POST("/test", func(c *gin.Context) {
		// Parse the encoding and decoding code type from the header
		coderType := c.Query("type")
		if coderType == "" || (coderType != codec.Json.String() && coderType != codec.Proto.String()) {
			coderType = codec.Json.String()
		}
		// 请求ID
		id := c.Query("target")
		if id == "" {
			id = "1232123"
		}
		// 读取 请求数据
		pkg, _ := io.ReadAll(c.Request.Body)

		rqId := c.Request.Header.Get("X-Request-ID")

		cc := moc.NewHttpConn(id, c.Request)
		connutil.SetHeartbeatInterval(cc, 60)
		// init
		if c.Query("tp") == "init" {
			responsePkg, _ := s.Handler().ProxyInit(cc, codec.Name(coderType), pkg)
			c.String(200, string(responsePkg))
			return
		}
		// package
		rqAction, respAction, _, _, responsePkg, err := s.Handler().Proxy(cc, rqId, pkg)
		if len(responsePkg) > 0 {
			c.String(200, string(responsePkg))
		}
		if err != nil {
			log.Println("rqAction:", rqAction.String(), "respAction", respAction.String(), "err:", err.Error())
		} else {
			log.Println("rqAction:", rqAction.String(), "respAction", respAction.String())
		}
	})

	s.Manager().ConnectionsListen(func(c *manage.Connection, b bool) {
		if b {
			log.Println("MANAGER>>>", c.Fd, " connected")
		} else {
			log.Println("MANAGER>>>", c.Fd, " disconnected")
		}
	})
	s.Manager().MessagesListen(func(message *manage.Message) {
		log.Println("MANAGER>>>", message.Fd, " ", message.Level, " ", message.Desc, string(message.Package))
	})
	app.AddServer(s)
	app.Run(func(err error) {
		panic(err)
	})
	app.Wait()
}
