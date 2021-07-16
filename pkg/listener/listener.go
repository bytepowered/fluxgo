package listener

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/internal"
	"github.com/bytepowered/fluxgo/pkg/logger"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	ConfigKeyAddress     = "address"
	ConfigKeyBindPort    = "bind_port"
	ConfigKeyTLSCertFile = "tls_cert_file"
	ConfigKeyTLSKeyFile  = "tls_key_file"
)

const (
	ConfigKeyFeatures             = "features"
	ConfigKeyFeatureBodyLimit     = "body_limit"
	ConfigKeyFeatureCORSEnable    = "cors_enable"
	ConfigKeyFeatureCSRFEnable    = "csrf_enable"
	ConfigKeyFeatureTrafficEnable = "traffic_enable"
)

var _ flux.WebListener = new(AdaptWebListener)

func init() {
	ext.SetWebListenerFactory(NewAdaptWebListener)
}

func NewAdaptWebListener(listenerId string, options *flux.Configuration) flux.WebListener {
	return NewAdaptWebListenerWith(listenerId, options, DefaultRequestIdentifierLocator, nil)
}

func NewAdaptWebListenerWith(listenerId string, options *flux.Configuration, identifier flux.WebRequestIdentityLocator, mws *AdaptMiddleware) flux.WebListener {
	flux.AssertNotEmpty(listenerId, "empty <listener-id> in web listener configuration")
	server := echo.New()
	server.Pre(RepeatableReadFilter)
	server.HideBanner = true
	server.HidePort = true
	webListener := &AdaptWebListener{
		id:           listenerId,
		server:       server,
		bodyResolver: DefaultRequestBodyResolver,
	}
	// Init context
	server.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(echoc echo.Context) error {
			id := identifier(echoc)
			flux.Assert("" != id, "<request-id> is empty, return by id lookup func")
			swc := NewWebContext(echoc, id, webListener)
			// Note: 不要修改echo.context.request对象引用，echo路由绑定了函数入口的request对象，
			// 从而导致后续基于request修改路由时，会导致Http路由失败。
			flux.AssertNil(echoc.Get(string(internal.CtxkeyWebContext)), "<web-context> must be nil")
			echoc.Set(string(internal.CtxkeyWebContext), swc)
			defer func() {
				if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {
					logger.Trace(id).Errorw("SERVER:CRITICAL:PANIC", "error", rvr, "error.trace", string(debug.Stack()))
					_ = echoc.JSON(http.StatusInternalServerError, map[string]interface{}{
						"server.traceid": id,
						"server.status":  "error",
						"error.level":    "critical",
						"error.message":  "unexpected fault of the server",
						"error.cause":    "internal error",
					})
				}
			}()
			return next(echoc)
		}
	})
	// Before feature
	if mws != nil && len(mws.BeforeFeature) > 0 {
		server.Pre(mws.BeforeFeature...)
	}

	// Feature
	features := options.Sub(ConfigKeyFeatures)
	// 是否设置BodyLimit
	if limit := features.GetString(ConfigKeyFeatureBodyLimit); "" != limit {
		logger.Infof("WebListener(id:%s), feature BODY-LIMIT: enabled, size= %s", webListener.id, limit)
		server.Pre(middleware.BodyLimit(limit))
	}
	// CORS
	if enabled := features.GetBool(ConfigKeyFeatureCORSEnable); enabled {
		logger.Infof("WebListener(id:%s), feature CORS: enabled", webListener.id)
		server.Pre(middleware.CORS())
	}
	// CSRF
	if enabled := features.GetBool(ConfigKeyFeatureCSRFEnable); enabled {
		logger.Infof("WebListener(id:%s), feature CSRF: enabled", webListener.id)
		server.Pre(middleware.CSRF())
	}
	// 流量日志
	if enabled := features.GetBool(ConfigKeyFeatureTrafficEnable); enabled {
		logger.Infof("WebListener(id:%s), feature TRAFFIC: enabled", webListener.id)
		webListener.AddFilter(NewTrafficVisitFilter())
	}
	// After features
	if mws != nil && len(mws.AfterFeature) > 0 {
		server.Pre(mws.AfterFeature...)
	}
	return webListener
}

// AdaptWebListener 默认实现的基于echo框架的WebServer
// 注意：保持AdaptWebServer的公共访问性
type AdaptWebListener struct {
	id           string
	server       *echo.Echo
	bodyResolver flux.WebBodyResolver
	tlsCertFile  string
	tlsKeyFile   string
	address      string
	started      bool
}

func (s *AdaptWebListener) ListenerId() string {
	return s.id
}

func (s *AdaptWebListener) OnInit(opts *flux.Configuration) error {
	logger.Infow("SERVER:EVENT:WEBLISTENER:INIT", "listener-id", s.id)
	s.tlsCertFile = opts.GetString(ConfigKeyTLSCertFile)
	s.tlsKeyFile = opts.GetString(ConfigKeyTLSKeyFile)
	addr, port := opts.GetString(ConfigKeyAddress), opts.GetString(ConfigKeyBindPort)
	if strings.Contains(addr, ":") {
		s.address = addr
	} else {
		s.address = addr + ":" + port
	}
	if s.address == ":" {
		return errors.New("web server config.address is required, was empty, listener-id: " + s.id)
	}
	flux.AssertNotNil(s.bodyResolver, "<body-resolver> is required, listener-id: "+s.id)
	return nil
}

func (s *AdaptWebListener) ListenServe() error {
	logger.Infow("SERVER:EVENT:WEBLISTENER:LISTEN", "listener-id", s.id, "address", s.address)
	s.started = true
	if "" != s.tlsCertFile && "" != s.tlsKeyFile {
		return s.server.StartTLS(s.address, s.tlsCertFile, s.tlsKeyFile)
	} else {
		return s.server.Start(s.address)
	}
}

func (s *AdaptWebListener) SetBodyResolver(r flux.WebBodyResolver) {
	flux.AssertNotNil(r, "<web-body-resolver> must not nil, listener-id: "+s.id)
	s.mustNotStarted().bodyResolver = r
}

func (s *AdaptWebListener) SetNotfoundHandler(f flux.WebHandlerFunc) {
	flux.AssertNotNil(f, "<notfound-handler> must not nil, listener-id: "+s.id)
	s.mustNotStarted().server.NotFoundHandler = AdaptWebHandler(f).AdaptFunc
}

func (s *AdaptWebListener) SetErrorHandler(handler flux.WebErrorHandlerFunc) {
	// Route请求返回的Error，全部经由此函数处理
	flux.AssertNotNil(handler, "<error-handler> must not nil, listener-id: "+s.id)
	s.mustNotStarted().server.HTTPErrorHandler = func(err error, c echo.Context) {
		if flux.IsNil(err) {
			return
		}
		webex, ok := c.Get(string(internal.CtxkeyWebContext)).(flux.WebContext)
		flux.Assert(ok, "<web-context> is invalid in http-error-handler")
		handler(webex, err)
	}
}

func (s *AdaptWebListener) HandleNotfound(webex flux.WebContext) error {
	return s.server.NotFoundHandler(webex.(*AdaptWebContext).ShadowContext())
}

func (s *AdaptWebListener) HandleError(webex flux.WebContext, err error) {
	s.server.HTTPErrorHandler(err, webex.(*AdaptWebContext).ShadowContext())
}

func (s *AdaptWebListener) AddFilter(i flux.WebFilter) {
	flux.AssertNotNil(i, "<web-filter> must not nil, listener-id: "+s.id)
	s.server.Pre(AdaptWebFilter(i).AdaptFunc)
}

func (s *AdaptWebListener) AddHandler(method, pattern string, h flux.WebHandlerFunc, is ...flux.WebFilter) {
	flux.AssertNotNil(h, "<web-handler> must not nil, listener-id: "+s.id)
	flux.AssertNotEmpty(method, "<http-method> must not empty")
	flux.AssertNotEmpty(pattern, "<http-pattern> must not empty")
	wms := make([]echo.MiddlewareFunc, len(is))
	for i, mi := range is {
		wms[i] = AdaptWebFilter(mi).AdaptFunc
	}
	s.server.Add(strings.ToUpper(method), s.resolve(pattern), AdaptWebHandler(h).AdaptFunc, wms...)
}

func (s *AdaptWebListener) AddHttpHandler(method, pattern string, h http.Handler, m ...func(http.Handler) http.Handler) {
	flux.AssertNotNil(h, "<web-handler> must not nil, listener-id: "+s.id)
	flux.AssertNotEmpty(method, "<http-method> must not empty")
	flux.AssertNotEmpty(pattern, "<http-pattern> must not empty")
	wms := make([]echo.MiddlewareFunc, len(m))
	for i, mf := range m {
		wms[i] = echo.WrapMiddleware(mf)
	}
	s.server.Add(strings.ToUpper(method), s.resolve(pattern), echo.WrapHandler(h), wms...)
}

func (s *AdaptWebListener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.server.ServeHTTP(w, r)
}

func (s *AdaptWebListener) ShadowRouter() interface{} {
	return s.server
}

func (s *AdaptWebListener) ShadowServer() interface{} {
	return s.server
}

func (s *AdaptWebListener) OnShutdown(ctx context.Context) error {
	s.started = false
	return s.server.Shutdown(ctx)
}

func (s *AdaptWebListener) mustNotStarted() *AdaptWebListener {
	flux.Assert(!s.started, "illegal state: web listener is started")
	return s
}

func (s *AdaptWebListener) resolve(pattern string) string {
	// /api/{userId} -> /api/:userId
	replaced := strings.Replace(pattern, "}", "", -1)
	if len(replaced) < len(pattern) {
		return strings.Replace(replaced, "{", ":", -1)
	} else {
		return pattern
	}
}

type AdaptMiddleware struct {
	BeforeFeature []echo.MiddlewareFunc
	AfterFeature  []echo.MiddlewareFunc
}
