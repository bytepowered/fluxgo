package listener

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/logger"
	"time"
)

func NewTrafficVisitFilter() flux.WebFilter {
	return func(next flux.WebHandlerFunc) flux.WebHandlerFunc {
		return func(webc flux.WebContext) error {
			multiWriter := NewEmptyHttpResponseMultiWriter(webc.ResponseWriter())
			webc.SetResponseWriter(multiWriter)
			defer func(trace flux.Logger, start time.Time) {
				trace.Infow("SERVER:TRAFFIC:VISIT",
					"listener-id", webc.WebListener().ListenerId(),
					"remote-ip", webc.RemoteAddr(),
					"request.uri", webc.URI(),
					"request.method", webc.Method(),
					"response.code", multiWriter.StatusCode(),
					"latency", time.Since(start).String(),
				)
			}(logger.Trace(webc.RequestId()), time.Now())
			return next(webc)
		}
	}
}
