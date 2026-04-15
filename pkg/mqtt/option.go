package mqtt

import "time"

type Option func(*Client)

func Auth(username, password string) Option {
	return func(c *Client) {
		c.username = username
		c.password = password
	}
}

func ClientId(id string) Option {
	return func(c *Client) {
		c.clientID = id
	}
}

func ReconnectInterval(iv time.Duration) Option {
	return func(c *Client) {
		if iv > 0 {
			c.recInterval = iv
		}
	}
}
