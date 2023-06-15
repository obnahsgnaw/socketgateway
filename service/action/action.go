package action

import (
	"github.com/obnahsgnaw/socketgateway/service/codec"
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
)

func New(id gatewayv1.ActionId) codec.Action {
	return codec.NewAction(codec.ActionId(id), id.String())
}
