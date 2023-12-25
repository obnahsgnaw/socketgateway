package main

import (
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
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strconv"
	"time"
)

func main() {
	app := application.New(
		application.NewCluster("dev", "Dev"),
		"demo",
		application.Debug(func() bool {
			return true
		}),
		application.Logger(&logger.Config{
			Dir:        "/Users/wangshanbo/Documents/Data/projects/socketgateway/out",
			MaxSize:    5,
			MaxBackup:  1,
			MaxAge:     1,
			Level:      "debug",
			TraceLevel: "error",
		}),
		application.EtcdRegister([]string{"127.0.0.1:2379"}, time.Second*5),
	)
	defer app.Release()

	s := socketgateway.New(app, sockettype.TCP, endtype.Frontend, url.Host{Ip: "127.0.0.1", Port: 8001})
	s.With(socketgateway.Engine(gnet.New()))
	l, _ := rpc2.NewListener(url.Host{Ip: "127.0.0.1", Port: 8002})
	rps := rpc2.New(app, l, "gw", "gw", endtype.Frontend)
	s.With(socketgateway.Rpc(rps, true))
	e, _ := http.Default(url.Host{Ip: "127.0.0.1", Port: 8003}, &engine.Config{
		Name:      "gw",
		DebugMode: false,
		LogDebug:  false,
		Cors:      nil,
		LogCnf:    nil,
	})
	s.With(socketgateway.Doc(e, "/v1/doc/socket/tcp", true))
	s.With(socketgateway.Watcher(func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field) {
		s.Logger().Debug(strconv.Itoa(c.Fd()) + ": " + msg)
	}))
	s.With(socketgateway.ReuseAddr())
	app.AddServer(s)
	app.Run(func(err error) {
		panic(err)
	})
	app.Wait()
}
