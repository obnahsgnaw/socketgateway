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
	"github.com/obnahsgnaw/socketgateway/pkg/socket/engine/net"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	s := socketgateway.New(app, sockettype.WSS, endtype.Frontend, url.Host{Ip: "127.0.0.1", Port: 8001})
	s.With(socketgateway.Engine(net.New()))
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
	app.AddServer(s)
	app.Run(func(err error) {
		panic(err)
	})
	app.Wait()
}
