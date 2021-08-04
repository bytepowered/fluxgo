package echo

import (
	ext "github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"io/ioutil"
	"net/http"
)

func init() {
	ext.RegisterTransporter(flux.ProtoEcho, NewTransporter())
}

var (
	_ flux.Transporter = new(RpcTransporter)
)

type RpcTransporter struct {
	codec flux.TransportCodecFunc
}

func NewTransporter() flux.Transporter {
	return &RpcTransporter{
		codec: NewTransportCodecFunc(),
	}
}

func (b *RpcTransporter) DoInvoke(context flux.Context, service flux.ServiceSpec) (*flux.ServeResponse, *flux.ServeError) {
	resp, err := b.invoke0(context, service)
	if err != nil {
		return nil, err
	}
	codec, _ := b.codec(context, resp, make(map[string]interface{}, 0))
	return codec, nil
}

func (b *RpcTransporter) invoke0(ctx flux.Context, service flux.ServiceSpec) (interface{}, *flux.ServeError) {
	var data []byte
	if r, err := ctx.BodyReader(); nil == err {
		data, _ = ioutil.ReadAll(r)
		_ = r.Close()
	}
	header := ctx.HeaderVars()
	return map[string]interface{}{
		"service":              service,
		"request-id":           ctx.RequestId(),
		"request-uri":          ctx.URI(),
		"request-method":       ctx.Method(),
		"request-pathValues":   ctx.PathVars(),
		"request-queryValues":  ctx.QueryVars(),
		"request-formValues":   ctx.FormVars(),
		"request-headerValues": header,
		"request-body":         string(data),
	}, nil
}

func NewTransportCodecFunc() flux.TransportCodecFunc {
	return func(ctx flux.Context, value interface{}, _ map[string]interface{}) (*flux.ServeResponse, error) {
		return &flux.ServeResponse{
			StatusCode: http.StatusOK,
			Headers:    make(http.Header, 0),
			Body:       value,
		}, nil
	}
}
