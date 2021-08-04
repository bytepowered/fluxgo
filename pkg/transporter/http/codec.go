package http

import (
	"errors"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"net/http"
)

var (
	ErrUnknownHttpResponse = errors.New("TRANSPORTER:HTTP:UNKNOWN_RESPONSE")
)

func NewTransportCodecFunc() flux.TransportCodecFunc {
	return func(ctx flux.Context, value interface{}, _ map[string]interface{}) (*flux.ServeResponse, error) {
		resp, ok := value.(*http.Response)
		if !ok {
			return &flux.ServeResponse{
				StatusCode: http.StatusBadGateway,
				Headers:    make(http.Header, 0),
				Body:       nil,
			}, ErrUnknownHttpResponse
		}
		return &flux.ServeResponse{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       resp.Body,
		}, nil
	}
}
