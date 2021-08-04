package inapp

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
	"io/ioutil"
)

func newDefaultInAppInvokeFunc() InvokeFunc {
	return func(ctx flux.Context, service flux.ServiceSpec) (interface{}, *flux.ServeError) {
		var data []byte
		if r, err := ctx.BodyReader(); nil == err {
			data, _ = ioutil.ReadAll(r)
			_ = r.Close()
		}
		header := ctx.HeaderVars()
		return map[string]interface{}{
			"service":              service,
			"biz-tag":              "inapp.echo",
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
}
