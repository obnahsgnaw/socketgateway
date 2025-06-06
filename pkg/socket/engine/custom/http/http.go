package http

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/obnahsgnaw/application/pkg/logging/logger"
	"github.com/obnahsgnaw/goutils/strutil"
	"github.com/obnahsgnaw/http"
	httpengine "github.com/obnahsgnaw/http/engine"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/limiter"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"io"
	http2 "net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Engine struct {
	ctx          context.Context
	s            *socket.Server
	eventHandler socket.Event
	t            sockettype.SocketType
	port         int
	config       *socket.Config
	index        int64
	connections  sync.Map // target ==> conn
	e            *http.Http
	ip           string
	limiter      *limiter.TimeLimiter
}

func New(ip string) *Engine {
	if ip == "" {
		ip = "0.0.0.0"
	}
	return &Engine{
		ip:      ip,
		limiter: limiter.NewTimeLimiter(10),
	}
}

func (e *Engine) genFd() int64 {
	atomic.AddInt64(&e.index, 1)
	return atomic.LoadInt64(&e.index)
}

// ?id=xxx
// ?id=xxx&at=device&dt=json&secret=
func (e *Engine) newHttp() (err error) {
	cnf := &httpengine.Config{}
	if cnf.AccessWriter == nil {
		cnf.AccessWriter, err = logger.NewDefAccessWriter(nil, func() bool {
			return true
		})
		if err != nil {
			return
		}
	}
	if cnf.ErrWriter == nil {
		cnf.ErrWriter, err = logger.NewDefErrorWriter(nil, func() bool {
			return true
		})
		if err != nil {
			return
		}
	}
	var s *http.Http
	s, err = http.Default(e.ip, e.port, cnf)
	if err != nil {
		return
	}

	s.Engine().POST("/", func(c *gin.Context) {
		target := c.Query("id")
		authType := c.Query("at")
		dataType := c.Query("dt")
		secret := c.Query("st")
		if dataType == "" {
			dataType = "json"
		}
		if target == "" {
			c.Writer.Header().Set("X-Error", "target is required")
			c.Status(http2.StatusBadRequest)
		}
		if !e.limiter.Access(target) {
			return
		}
		var conn *Conn
		if v, ok := e.connections.Load(target); !ok {
			conn = NewConn(target, int(e.genFd()), c.Request, func(id string) {
				if v1, ok1 := e.connections.Load(target); ok1 {
					v2 := v1.(*Conn)
					e.connections.Delete(id)
					e.eventHandler.OnClose(e.s, v2, nil)
				}
			})
			e.connections.Store(target, conn)
			e.eventHandler.OnOpen(e.s, conn)
			conn.Context().SetOptional("fd-target", target)
		} else {
			conn = v.(*Conn)
		}
		conn.Context().Active()

		if conn.Doing() {
			return
		}

		// 可以query携带认证信息， 但默认时不加密情况
		if authType != "" && !conn.Context().Authenticated() {
			conn.SetRq([]byte(strutil.ToString(authType, "@", target, "@", dataType, "::", secret)))
			e.eventHandler.OnTraffic(e.s, conn)
			time.Sleep(time.Millisecond * 500)
			respData := conn.GetResp()
			if string(respData) == "222" {
				c.Writer.Header().Set("X-Error", "authentication failed")
				c.Status(http2.StatusBadRequest)
				e.limiter.Hit(target)
				return
			}
			e.limiter.Release(target)
			c.Writer.Header().Set("X-Error", string(respData))
		}

		rqData, _ := io.ReadAll(c.Request.Body)
		var respData []byte
		if len(rqData) > 0 {
			conn.SetRq(rqData)
			e.eventHandler.OnTraffic(e.s, conn)
			time.Sleep(time.Millisecond * 100)
			respData = conn.GetResp()
		}
		c.String(http2.StatusOK, string(respData))
	})
	e.e = s

	return nil
}

func (e *Engine) Stop() error {
	e.e.Close()
	e.eventHandler.OnShutdown(e.s)
	return nil
}

func (e *Engine) Run(ctx context.Context, s *socket.Server, eventHandler socket.Event, t sockettype.SocketType, port int, config *socket.Config) (err error) {
	if t != sockettype.HTTP {
		return errors.New("the socket custom engin only support the htt type")
	}
	e.ctx = ctx
	e.s = s
	e.eventHandler = eventHandler
	e.t = t
	e.config = config
	e.port = port
	if err = e.newHttp(); err != nil {
		return err
	}
	e.eventHandler.OnBoot(e.s)
	e.tick()
	return e.e.RunAndServ()
}

func (e *Engine) tick() {
	if e.config.Ticker {
		go func(ctxx context.Context) {
			for {
				select {
				case <-ctxx.Done():
					break
				default:
					delay := e.eventHandler.OnTick(e.s)
					time.Sleep(delay)
				}
			}
		}(e.ctx)
	}
}
