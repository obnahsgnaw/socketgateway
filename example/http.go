package main

import (
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/application/service/regCenter"
	"github.com/obnahsgnaw/http"
	"github.com/obnahsgnaw/http/engine"
	rpc2 "github.com/obnahsgnaw/rpc"
	"github.com/obnahsgnaw/socketgateway"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/manage"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"strconv"
	"time"
)

func main() {
	reg, _ := regCenter.NewEtcdRegister([]string{"127.0.0.1:2379"}, time.Second*5)
	app := application.New(
		"demo",
		application.Debug(func() bool {
			return true
		}),
		application.Register(reg, 5),
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

	s := socketgateway.New(app, sockettype.HTTP, endtype.Frontend, url.Host{Ip: "127.0.0.1", Port: 8004}, "outer")
	l, _ := rpc2.NewListener(url.Host{Ip: "127.0.0.1", Port: 8005})
	rps := rpc2.New(app, l, "gw", "gw", endtype.Frontend, nil, rpc2.RegEnable())
	s.With(socketgateway.Rpc(rps))
	e, _ := http.Default("127.0.0.1", 8006, &engine.Config{
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
		Id:   1,
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
	s.With(socketgateway.Tick(time.Second * 1))
	s.With(socketgateway.Heartbeat(time.Second * 10))
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
