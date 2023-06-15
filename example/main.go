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
)

func main() {
	app := application.New("demo", "demo")

	s := socketgateway.New(app, sockettype.TCP, endtype.Backend, url.Host{Ip: "127.0.0.1", Port: 8001})
	s.WithRpcServer(8002)
	s.WithDocServer("", 8003)
	s.WatchLog(func(c socket.Conn, msg string, l zapcore.Level, data ...zap.Field) {
		log.Println(msg)
	})

	app.AddServer(s)
	app.Run(func(err error) {
		panic(err)
	})
	log.Println("Exited")
}
