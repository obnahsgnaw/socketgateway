package mqtt

import (
	"context"
	"errors"
	"fmt"
	"github.com/obnahsgnaw/goutils/strutil"
	messagev1 "github.com/obnahsgnaw/socketapi/gen/message/v1"
	"github.com/obnahsgnaw/socketgateway/pkg/mqtt"
	"github.com/obnahsgnaw/socketgateway/pkg/socket"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/limiter"
	"github.com/obnahsgnaw/socketgateway/pkg/socket/sockettype"
	"github.com/obnahsgnaw/socketgateway/service/eventhandler/connutil"
	"github.com/obnahsgnaw/socketutil/codec"
	"google.golang.org/protobuf/proto"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const topicIdKey = "device_sn"
const topicActionKey = "action"           // no action is raw pkg
const authenticateAction = "authenticate" // authenticate action

type Engine struct {
	ctx          context.Context
	s            *socket.Server
	eventHandler socket.Event
	t            sockettype.SocketType
	port         int
	config       *socket.Config
	index        int64
	connections  sync.Map // target ==> *Conn
	client       *mqtt.Client
	topics       []QosTopic
	serverTopic  *QosTopic
	limiter      *limiter.TimeLimiter
}

type QosTopic struct {
	Topic string
	Qos   mqtt.Qos
}

func New(addr string, o ...mqtt.Option) *Engine {
	return &Engine{
		client:  mqtt.New(addr, o...),
		limiter: limiter.NewTimeLimiter(10),
	}
}

// ListenTopic xxx/{device_sn}/xxx/{action}/client , xxx/{device_sn}/xxx/{action}/server
func (e *Engine) ListenTopic(clientTopic, serverTopic QosTopic) error {
	if !strings.Contains(clientTopic.Topic, "{"+topicIdKey+"}") || !strings.Contains(clientTopic.Topic, "{"+topicActionKey+"}") {
		return fmt.Errorf("invalid client topic: %s, not contains {%s} to parse device SN and {%s} to parse action", clientTopic.Topic, topicIdKey, topicActionKey)
	}
	e.topics = append(e.topics, clientTopic)
	if !strings.Contains(serverTopic.Topic, "{"+topicIdKey+"}") || !strings.Contains(serverTopic.Topic, "{"+topicActionKey+"}") {
		return fmt.Errorf("invalid server topic: %s, not contains {%s} to parse device SN and {%s} to parse action", serverTopic.Topic, topicIdKey, topicActionKey)
	}
	e.serverTopic = &serverTopic
	return nil
}

func (e *Engine) ListenRawTopic(topic ...QosTopic) error {
	for _, t := range topic {
		if !strings.Contains(t.Topic, "{"+topicIdKey+"}") {
			return fmt.Errorf("invalid topic: %s, not contains {%s} to parse device SN", t.Topic, topicIdKey)
		}
		e.topics = append(e.topics, t)
	}
	return nil
}

func (e *Engine) genFd() int64 {
	atomic.AddInt64(&e.index, 1)
	return atomic.LoadInt64(&e.index)
}

func (e *Engine) Stop() error {
	e.client.Disconnect()
	e.eventHandler.OnShutdown(e.s)
	return nil
}

func (e *Engine) Run(ctx context.Context, s *socket.Server, eventHandler socket.Event, t sockettype.SocketType, port int, config *socket.Config) (err error) {
	if t != sockettype.MQTT {
		return errors.New("the socket custom engin only support the htt type")
	}
	e.ctx = ctx
	e.s = s
	e.eventHandler = eventHandler
	e.t = t
	e.config = config
	e.port = port

	e.client.Message(func(client *mqtt.Client, message mqtt.Message) {
		if sn := message.Param(topicIdKey); sn != "" {
			if !e.limiter.Access(sn) {
				return
			}
			var conn *Conn
			var raw bool
			if v, ok := e.connections.Load(sn); !ok {
				conn = NewConn(sn, int(e.genFd()), func(id string) {
					if v1, ok1 := e.connections.Load(sn); ok1 {
						v2 := v1.(*Conn)
						e.connections.Delete(id)
						e.eventHandler.OnClose(e.s, v2, nil)
					}
				}, func(cc *Conn, wb []byte) error {
					if e.isRawConn(cc) {
						str := string(wb)
						if str == "000" || str == "111" || str == "222" {
							return nil
						}
						var msg messagev1.MqttPackage
						if err1 := proto.Unmarshal(wb, &msg); err1 != nil {
							return err1
						}
						if msg.Topic == "" {
							return errors.New("invalid mqtt topic")
						}
						return e.client.Publish(msg.Topic, mqtt.Qos(msg.Qos), msg.Retained, msg.Payload)
					}
					str := string(wb)
					if str == "000" || str == "111" || str == "222" {
						return e.response(cc.id, authenticateAction, wb)
					}
					pkgBuilder := connutil.GatewayPkgCoder(cc)
					var pkg *codec.PKG
					if pkg, err = pkgBuilder.Unpack(wb); err != nil {
						return errors.New("encode gateway pkg failed")
					}
					return e.response(cc.id, strconv.Itoa(int(pkg.Action)), pkg.Data)
				})
				raw = e.isRawMessage(message)
				conn.Context().SetOptional("mqtt_raw", raw)
				conn.Context().SetOptional("fd-target", sn)
				e.connections.Store(sn, conn)
				e.eventHandler.OnOpen(e.s, conn)
			} else {
				conn = v.(*Conn)
				raw = e.isRawConn(conn)
			}
			conn.Context().Active()
			if !raw {
				action := message.Param(topicActionKey)
				if action == authenticateAction {
					conn.SetRq(message.Payload())
					e.eventHandler.OnTraffic(e.s, conn)
					return
				}
				if conn.Context().Authenticated() {
					actionId, _ := strconv.Atoi(action)
					if actionId > 0 {
						pkg := &codec.PKG{
							Action: codec.ActionId(actionId),
							Data:   message.Payload(),
						}
						pkgBuilder := connutil.GatewayPkgCoder(conn)
						var b []byte
						if b, err = pkgBuilder.Pack(pkg); err == nil {
							conn.SetRq(b)
							e.eventHandler.OnTraffic(e.s, conn)
						}
					}
				}
				return
			}
			if !conn.Context().Authenticated() {
				conn.SetRq([]byte(strutil.ToString("device@", sn, "@proto::")))
				e.eventHandler.OnTraffic(e.s, conn)
				time.Sleep(time.Millisecond * 500)
			}
			if !conn.Context().Authenticated() {
				e.limiter.Hit(sn)
				return
			}
			e.limiter.Release(sn)
			pkg := &messagev1.MqttPackage{
				Duplicate: message.Duplicate(),
				Qos:       int32(message.Qos()),
				Retained:  message.Retained(),
				Topic:     message.Topic(),
				MessageId: uint32(message.MessageID()),
				Payload:   message.Payload(),
			}
			pkgBytes, err2 := proto.Marshal(pkg)
			if err2 == nil {
				conn.SetRq(pkgBytes)
				e.eventHandler.OnTraffic(e.s, conn)
			}
		}
	})
	if err = e.client.Connect(); err != nil {
		return err
	}
	for _, topic := range e.topics {
		if err = e.client.Subscribe(topic.Topic, topic.Qos, nil); err != nil {
			return err
		}
	}

	e.tick()

	e.eventHandler.OnBoot(e.s)
	select {}
}

func (e *Engine) isRawMessage(m mqtt.Message) bool {
	return e.serverTopic == nil || m.Param(topicActionKey) == ""
}

func (e *Engine) isRawConn(conn *Conn) bool {
	rawV, _ := conn.Context().GetOptional("mqtt_raw")
	raw, _ := rawV.(bool)
	return raw
}

func (e *Engine) response(sn string, action string, payload []byte) error {
	topic := strings.Replace(e.serverTopic.Topic, strutil.ToString("{", topicIdKey, "}"), sn, 1)
	topic = strings.Replace(topic, strutil.ToString("{", topicActionKey, "}"), action, 1)
	return e.client.Publish(topic, e.serverTopic.Qos, false, payload)
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
