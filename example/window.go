package main

import (
	"github.com/obnahsgnaw/application"
	"github.com/obnahsgnaw/application/endtype"
	"github.com/obnahsgnaw/application/pkg/url"
	"github.com/obnahsgnaw/socketgateway"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"time"
)

func main() {
	app := application.New(application.NewCluster("dev", "Dev"), "socketGwDemo")
	defer app.Release()
	app.With(application.Debug(func() bool {
		return true
	}))
	app.With(application.EtcdRegister([]string{"127.0.0.1:2379"}, time.Second*5))

	s := socketgateway.New(app, sockettype.TCP, endtype.Frontend, url.Host{Ip: "127.0.0.1", Port: 8001})
	s.WithRpcServer(8002)
	s.WithDocServer(8003, "/v1/doc/socket/tcp")
	s.WatchLog(func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field) {
		log.Println(msg)
	})

	app.AddServer(s)
	app.Run(func(err error) {
		panic(err)
	})
	app.Wait()
}
