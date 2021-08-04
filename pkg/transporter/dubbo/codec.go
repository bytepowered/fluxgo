package dubbo

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/spf13/cast"
	"net/http"
)

const (
	ResponseKeyStatusCode = "@net.bytepowered.flux/http.status"
	ResponseKeyHeaders    = "@net.bytepowered.flux/http.headers"
)

func NewTransportCodecFuncWith(codeKey, headerKey string) flux.TransportCodecFunc {
	return func(ctx flux.Context, body interface{}, att map[string]interface{}) (*flux.ServeResponse, error) {
		attrs := make(map[string]interface{}, 8)
		// 从Attachment中读取StatusCode，HeaderMap
		status := flux.StatusOK
		header := make(http.Header)
		for k, v := range att {
			if k == codeKey {
				status = cast.ToInt(v)
			} else if k == headerKey {
				for hk, hv := range cast.ToStringMap(v) {
					header.Set(hk, cast.ToString(hv))
				}
			} else {
				attrs[k] = v
			}
		}
		return &flux.ServeResponse{StatusCode: status, Headers: header, Attachments: attrs, Body: body}, nil
	}
}

func NewTransportCodecFunc() flux.TransportCodecFunc {
	return NewTransportCodecFuncWith(ResponseKeyStatusCode, ResponseKeyHeaders)
}
