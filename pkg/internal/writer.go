package internal

import (
	ext "github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/spf13/cast"
)

var _ flux.ServeResponseWriter = new(JSONServeResponseWriter)

// JSONServeResponseWriter 默认输出JSON结构响应数据的Writer
type JSONServeResponseWriter int

func (r *JSONServeResponseWriter) Write(ctx flux.Context, response *flux.ServeResponse) {
	header := ctx.ResponseWriter().Header()
	for k, hv := range response.Headers {
		for _, v := range hv {
			header.Add(k, v)
		}
	}
	if bytes, err := ext.JSONMarshalObject(response.Body); nil != err {
		r.WriteError(ctx, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			Message:    "GATEWAY:DECODE:RESPONSE/error",
			CauseError: err,
		})
	} else {
		r.write(ctx, response.StatusCode, bytes)
	}
}

func (r *JSONServeResponseWriter) WriteError(ctx flux.Context, err *flux.ServeError) {
	bytes, _ := ext.JSONMarshalObject(map[string]interface{}{
		"status":  "error",
		"code":    err.ErrorCode,
		"message": err.Message,
		"error":   cast.ToString(err.CauseError),
	})
	r.write(ctx, err.StatusCode, bytes)
}

func (r *JSONServeResponseWriter) write(ctx flux.Context, status int, body []byte) {
	ctx.ResponseWriter().Header().Add("X-Writer-Id", "Fx-TWriter")
	err := ctx.Write(status, flux.MIMEApplicationJSONCharsetUTF8, body)
	if nil != err {
		ctx.Logger().Errorw("RESP-WRITER:WRITE:ERROR", "error", err)
	} else {
		ctx.Logger().Infow("RESP-WRITER:WRITE:COMPLETED", "body", string(body))
	}
}
