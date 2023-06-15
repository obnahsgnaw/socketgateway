package codec

import (
	"github.com/obnahsgnaw/application/pkg/security"
)

func NewEsCrypto(esType security.EsType, mode security.EsMode) *security.EsCrypto {
	return security.NewEsCrypto(esType, mode)
}

func GenerateEsKey(esType security.EsType) []byte {
	return esType.RandKey()
}

func NewRsaCrypto() *security.RsaCrypto {
	return security.NewRsa()
}
