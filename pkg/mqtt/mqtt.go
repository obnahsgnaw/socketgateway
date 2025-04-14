package mqtt

import (
	"errors"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"regexp"
)

type Qos byte

func (q Qos) Val() byte {
	return byte(q)
}

const (
	QoS0 Qos = 0
	QoS1 Qos = 1
	QoS2 Qos = 2
)

type Client struct {
	client     mqtt.Client
	address    string
	username   string
	password   string
	clientID   string
	msgHandler MessageHandler
}

type Message interface {
	mqtt.Message
	Param(key string) string
}

type message struct {
	mqtt.Message
	params map[string]string
}

func newMessage(msg mqtt.Message, re *ParamPattern) Message {
	return message{
		Message: msg,
		params:  re.Parse(msg.Topic()),
	}
}

func (m message) Param(key string) string {
	if v, ok := m.params[key]; ok {
		return v
	}
	return ""
}

type MessageHandler func(*Client, Message)

// New tcpAddr tcp://ip:port
func New(tcpAddr string, o ...Option) *Client {
	s := &Client{
		address: tcpAddr,
	}
	s.With(o...)
	return s
}

func (c *Client) With(o ...Option) {
	for _, fn := range o {
		fn(c)
	}
}

func (c *Client) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.address)
	opts.SetClientID(c.ClientId())
	if c.username != "" {
		opts.SetUsername(c.username)
		opts.SetPassword(c.password)
	}
	opts.OnConnect = func(client mqtt.Client) {

	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {

	}
	opts.DefaultPublishHandler = func(client mqtt.Client, msg mqtt.Message) {
		if c.msgHandler != nil {
			c.msgHandler(c, newMessage(msg, nil))
		}
	}
	c.client = mqtt.NewClient(opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (c *Client) Publish(topic string, qos Qos, retained bool, payload interface{}) error {
	if c.client == nil {
		return errors.New("client is not connected")
	}
	token := c.client.Publish(topic, qos.Val(), retained, payload)
	token.Wait()
	return token.Error()
}

// Subscribe support param topic for example: xxx/{param}/xxx/{param}, callback can nil
func (c *Client) Subscribe(topic string, qos Qos, callback MessageHandler) error {
	if c.client == nil {
		return errors.New("client is not connected")
	}
	re, err := NewParamPattern(topic)
	if err != nil {
		return errors.New("invalid topic")
	}
	token := c.client.Subscribe(c.convertTopic(topic), byte(qos), func(client mqtt.Client, message mqtt.Message) {
		if callback != nil {
			callback(c, newMessage(message, re))
			return
		}
		if c.msgHandler != nil {
			c.msgHandler(c, newMessage(message, re))
		}
	})
	token.Wait()
	return token.Error()
}

// Message Subscribe callback nil message will to this
func (c *Client) Message(fn MessageHandler) {
	c.msgHandler = fn
}

func (c *Client) ClientId() string {
	if c.clientID != "" {
		return c.clientID
	}
	return "go_mqtt_client"
}

func (c *Client) Disconnect() {
	if c.client != nil {
		c.client.Disconnect(250)
	}
}

func (c *Client) convertTopic(pattern string) string {
	return regexp.MustCompile(`{(\w+)}`).ReplaceAllStringFunc(pattern, func(param string) string {
		return "+"
	})
}
