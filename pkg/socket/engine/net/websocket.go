package net

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
)

type wssEngineHandler struct {
	e        *Engine
	port     int
	upgrader websocket.Upgrader
}

func newWssEngineHandler(e *Engine, port int) *wssEngineHandler {
	return &wssEngineHandler{
		e:    e,
		port: port,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  2048,
			WriteBufferSize: 2048,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *wssEngineHandler) Init() error {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := h.upgrader.Upgrade(writer, request, nil)
		if err != nil {
			_, _ = writer.Write([]byte(err.Error()))
		}
		defer func() {
			_ = conn.Close()
		}()

		atomic.AddInt64(&h.e.index, 1)
		fd := atomic.LoadInt64(&h.e.index)
		c := newWssConn(int(fd), conn, socket.NewContext())
		h.e.connections.Store(fd, c)
		h.e.event.OnOpen(h.e.server, c)
		h.handConn(c)
	})

	return nil
}

func (h *wssEngineHandler) Run() {
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(h.port), nil))
}

func (h *wssEngineHandler) handConn(c *WssConn) {
	defer func() {
		c.Close()
		h.e.event.OnClose(h.e.server, c, errors.New("close by peer"))
		h.e.connections.Delete(c.fd)
	}()
	for {
		_, b, err := c.raw.ReadMessage()
		if err != nil {
			break
		}
		if len(b) > 0 {
			c.pkg = append(c.pkg, b)
			c.Context().Active()
			h.e.event.OnTraffic(h.e.server, c)
		}
	}
}
