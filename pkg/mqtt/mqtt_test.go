package mqtt

import (
	"log"
	"testing"
)

func TestMqtt(t *testing.T) {
	c := New("127.0.0.1:1883", Auth("test", "123456"))
	err := c.Connect()
	if err != nil {
		t.Error(err)
		return
	}
	err = c.Subscribe("testtopic/+", QoS0, func(client *Client, message Message) {
		log.Println(message.Topic(), string(message.Payload()))
	})
	if err != nil {
		t.Error(err)
		return
	}
	log.Println("Serving...")
	select {}
}

func TestMqttParam(t *testing.T) {
	c := New("127.0.0.1:1883", Auth("test", "123456"))
	err := c.Connect()
	if err != nil {
		t.Error(err)
		return
	}
	err = c.Subscribe("testtopic/{id}", QoS0, func(client *Client, message Message) {
		log.Println(message.Topic(), message.Param("id"), string(message.Payload()))
	})
	if err != nil {
		t.Error(err)
		return
	}
	log.Println("Serving...")
	select {}
}

func TestMqttMessage(t *testing.T) {
	c := New("127.0.0.1:1883", Auth("test", "123456"))
	err := c.Connect()
	if err != nil {
		t.Error(err)
		return
	}
	err = c.Subscribe("testtopic/{id}", QoS0, nil)
	if err != nil {
		t.Error(err)
		return
	}
	c.Message(func(client *Client, message Message) {
		log.Println("topic:", message.Topic(), "id:", message.Param("id"), "payload:", string(message.Payload()))
	})
	log.Println("Serving...")
	select {}
}

func TestParamPattern(t *testing.T) {
	pattern := `thing/product/{device_sn}/state`
	p, _ := NewParamPattern(pattern)
	rs := p.Parse("thing/product/abcd/state")
	log.Println(rs)
}
