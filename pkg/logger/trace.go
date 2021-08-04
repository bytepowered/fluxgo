package logger

import (
	"context"
	ext "github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"strings"
)

const (
	TraceId = "trace-id"
	Extras  = "extras"
)

func Trace(id string) flux.Logger {
	return ext.NewLoggerWith(context.WithValue(context.Background(), TraceId, id))
}

func TraceVerbose(ctx flux.Context) flux.Logger {
	return TraceVerboseExtras(ctx, nil)
}

func TraceVerboseExtras(ctx flux.Context, extras map[string]string) flux.Logger {
	flux.AssertNotNil(ctx, "<flux.context> must not nil in log trace")
	fields := map[string]string{
		"request.method": ctx.Method(),
		"request.uri":    ctx.URI(),
	}
	for k, v := range extras {
		fields[k] = v
	}
	endpoint := ctx.Endpoint()
	if nil != endpoint && endpoint.IsValid() {
		fields["endpoint.appid"] = endpoint.Application
		fields["endpoint.bizkey"] = endpoint.Annotation(flux.EndpointAnnotationBizKey).GetString()
		fields["endpoint.version"] = endpoint.Version
		fields["endpoint.pattern"] = endpoint.HttpPattern
		fields["endpoint.authorize"] = endpoint.Annotation(flux.EndpointAnnotationAuthorize).GetString()
		fields["endpoint.service"] = endpoint.Service.ServiceID()
		fields["endpoint.permissions"] = strings.Join(endpoint.Annotation(flux.EndpointAnnotationPermissions).GetStrings(), ",")
	}
	return TraceExtras(ctx.RequestId(), fields)
}

func TraceExtras(traceId string, extras map[string]string) flux.Logger {
	p := context.WithValue(context.Background(), TraceId, traceId)
	return ext.NewLoggerWith(context.WithValue(p, Extras, extras))
}
