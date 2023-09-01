package eventhandler

import "github.com/obnahsgnaw/application/pkg/security"

type Cryptor interface {
	Type() security.EsType
	Encrypt(data, key []byte, b64 bool) (encrypted, iv []byte, err error)
	Decrypt(encrypted, key, iv []byte, b64 bool) (data []byte, err error)
}
