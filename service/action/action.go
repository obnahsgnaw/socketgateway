package action

import (
	gatewayv1 "github.com/obnahsgnaw/socketgateway/service/proto/gen/gateway/v1"
	"github.com/obnahsgnaw/socketutil/codec"
)

func New(id gatewayv1.ActionId) codec.Action {
	return codec.NewAction(codec.ActionId(id), id.String())
}
