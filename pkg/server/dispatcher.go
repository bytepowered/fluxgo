package server

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

import (
	"github.com/jinzhu/copier"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/internal"
	"github.com/bytepowered/fluxgo/pkg/logger"
)

type (
	DispatcherOptionFunc func(d *Dispatcher)
)

type Dispatcher struct {
	flux.WebListener
	metrics                *Metrics
	pooled                 *sync.Pool
	responseWriter         flux.ServeResponseWriter
	versionLocator         flux.WebRequestVersionLocator
	onContextHooks         []flux.OnContextHookFunc
	onBeforeFilterHooks    []flux.OnBeforeFilterHookFunc
	onBeforeTransportHooks []flux.OnBeforeTransportHookFunc
}

// WithRequestVersionLocator 配置Web请求版本选择函数
func WithRequestVersionLocator(fun flux.WebRequestVersionLocator) DispatcherOptionFunc {
	return func(d *Dispatcher) {
		d.SetVersionLocator(fun)
	}
}

// WithServeResponseWriter 配置ResponseWriter
func WithServeResponseWriter(writer flux.ServeResponseWriter) DispatcherOptionFunc {
	return func(d *Dispatcher) {
		d.SetResponseWriter(writer)
	}
}

// WithOnBeforeTransportHookFunc 配置Transporter调用前的钩子函数列表
func WithOnBeforeTransportHookFunc(hooks ...flux.OnBeforeTransportHookFunc) DispatcherOptionFunc {
	return func(d *Dispatcher) {
		for _, h := range hooks {
			d.AddOnBeforeTransportHook(h)
		}
	}
}

// WithOnContextHooks 配置WebContext转换为fluxContext时后的钩子函数列表
func WithOnContextHooks(hooks ...flux.OnContextHookFunc) DispatcherOptionFunc {
	return func(d *Dispatcher) {
		for _, h := range hooks {
			d.AddOnContextHook(h)
		}
	}
}

func newDispatcher(listener flux.WebListener) *Dispatcher {
	return &Dispatcher{
		WebListener:            listener,
		metrics:                NewMetricsWith(listener.ListenerId()),
		pooled:                 &sync.Pool{New: func() interface{} { return internal.NewContext() }},
		versionLocator:         DefaultRequestVersionLocateFunc,
		responseWriter:         new(internal.JSONServeResponseWriter),
		onContextHooks:         make([]flux.OnContextHookFunc, 0, 4),
		onBeforeFilterHooks:    make([]flux.OnBeforeFilterHookFunc, 0, 4),
		onBeforeTransportHooks: make([]flux.OnBeforeTransportHookFunc, 0, 4),
	}
}

func (d *Dispatcher) route(webex flux.WebContext, versions *flux.MVCEndpoint) (err error) {
	defer func(id string) {
		if panerr := recover(); panerr != nil {
			trace := logger.Trace(id)
			if recerr, ok := panerr.(error); ok {
				trace.Errorw(recerr.Error(), "r-error", recerr, "debug", string(debug.Stack()))
				err = recerr
			} else {
				trace.Errorw("DISPATCH:EVEN:ROUTE:CRITICAL_PANIC", "r-error", panerr, "debug", string(debug.Stack()))
				err = fmt.Errorf("DISPATCH:EVEN:ROUTE:%s", panerr)
			}
		}
	}(webex.RequestId())
	var endpoint flux.EndpointSpec
	// 查找匹配版本的Endpoint
	if src, found := d.lookup(webex, d.WebListener, versions); found {
		// dup to enforce metadata safe
		cperr := d.dup(&endpoint, src)
		flux.AssertM(cperr == nil, func() string {
			return fmt.Sprintf("duplicate endpoint metadata, error: %s", cperr.Error())
		})
	} else {
		logger.Trace(webex.RequestId()).Infow("DISPATCH:EVEN:ROUTE:ENDPOINT/not-found",
			"http-pattern", []string{webex.Method(), webex.URI(), webex.URL().Path},
		)
		// Endpoint节点版本被删除，需要重新路由到NotFound处理函数
		return d.WebListener.HandleNotfound(webex)
	}
	// check endpoint bindings
	flux.AssertTrue(endpoint.IsValid(), "<endpoint> must valid when routing")
	flux.AssertTrue(endpoint.Service.IsValid(), "<endpoint.service> must valid when routing")
	ctxw := d.pooled.Get().(flux.Context)
	defer d.pooled.Put(ctxw)
	ctxw.(*internal.Context).Reset(webex, &endpoint)
	ctxw.SetAttribute(flux.XRequestTime, ctxw.StartAt().Unix())
	ctxw.SetAttribute(flux.XRequestId, ctxw.RequestId())
	logger.TraceVerbose(ctxw).Infow("DISPATCH:EVEN:ROUTE:START")
	defer func(start time.Time) {
		logger.Trace(webex.RequestId()).Infow("DISPATCH:EVEN:ROUTE:END", "metric", ctxw.Metrics(), "elapses", time.Since(start).String())
	}(ctxw.StartAt())
	// context hook
	for _, hook := range d.onContextHooks {
		hook(webex, ctxw)
	}
	// route and dispatch
	return d.dispatch(ctxw)
}

func (d *Dispatcher) dispatch(ctx flux.Context) *flux.ServeError {
	// Metric: Route
	defer func() {
		ctx.AddMetric("dispatcher", time.Since(ctx.StartAt()))
	}()
	ctx.AddMetric("selector", time.Since(ctx.StartAt()))
	// Walk filters
	filters := d.selectFilters(ctx)
	for _, before := range d.onBeforeFilterHooks {
		before(ctx, filters)
	}
	next := func(ctx flux.Context) *flux.ServeError {
		ctx.AddMetric("filters", time.Since(ctx.StartAt()))
		if perr := d.handlePlugins(ctx); perr != nil {
			return perr
		}
		return d.doTransport(ctx)
	}
	// fin: transport and write data (response,error)
	if ferr := d.makeFilterChain(next, filters)(ctx); ferr != nil {
		d.responseWriter.WriteError(ctx, ferr)
	}
	return nil // always return nil
}

func (d *Dispatcher) makeFilterChain(next flux.FilterInvoker, filters []flux.Filter) flux.FilterInvoker {
	for i := len(filters) - 1; i >= 0; i-- {
		next = filters[i].DoFilter(next)
	}
	return next
}

func (d *Dispatcher) lookup(webex flux.WebContext, server flux.WebListener, endpoints *flux.MVCEndpoint) (*flux.EndpointSpec, bool) {
	// 动态Endpoint版本选择
	for _, selector := range ext.EndpointSelectors() {
		if selector.Active(webex, server.ListenerId()) {
			if ep, ok := selector.DoSelect(webex, server.ListenerId(), endpoints); ok {
				return ep, true
			}
		}
	}
	// 默认版本选择
	return endpoints.Lookup(d.versionLocator(webex))
}

func (d *Dispatcher) handlePlugins(ctx flux.Context) *flux.ServeError {
	defer func() {
		ctx.AddMetric("plugins", time.Since(ctx.StartAt()))
	}()
	for _, plugin := range d.selectPlugins(ctx) {
		timer := d.metrics.NewRouteVecTimer("Plugin", plugin.PluginId())
		pherr := plugin.DoHandle(ctx)
		timer.ObserveDuration()
		if pherr != nil {
			return pherr
		}
		ctx.AddMetric(plugin.PluginId(), time.Since(ctx.StartAt()))
	}
	return nil
}

func (d *Dispatcher) doTransport(ctx flux.Context) *flux.ServeError {
	select {
	case <-ctx.Context().Done():
		return &flux.ServeError{StatusCode: flux.StatusBadRequest,
			ErrorCode: "DISPATCHER:TRANSPORT:CANCELED/100", CauseError: ctx.Context().Err(),
		}
	default:
		break
	}
	defer func() {
		ctx.AddMetric("transporter", time.Since(ctx.StartAt()))
	}()
	proto := ctx.Service().Protocol
	transporter, ok := ext.TransporterByProto(proto)
	if !ok {
		logger.TraceVerbose(ctx).Errorw("DISPATCH:EVEN:ROUTE:UNSUPPORTED_PROTOCOL",
			"proto", proto, "service", ctx.Endpoint().Service)
		return &flux.ServeError{StatusCode: flux.StatusNotFound,
			ErrorCode: flux.ErrorCodeRequestNotFound,
			Message:   fmt.Sprintf("SERVER:ROUTE:ILLEGAL_PROTOCOL/%s", proto),
		}
	}
	for _, hook := range d.onBeforeTransportHooks {
		hook(ctx, transporter)
	}
	timer := d.metrics.NewRouteVecTimer("Transporter", proto)
	invret, inverr := transporter.DoInvoke(ctx, ctx.Service())
	timer.ObserveDuration()
	select {
	case <-ctx.Context().Done():
		return &flux.ServeError{StatusCode: flux.StatusBadRequest,
			ErrorCode: "DISPATCHER:TRANSPORT:CANCELED/200", CauseError: ctx.Context().Err(),
		}
	default:
		if flux.IsNil(inverr) {
			d.responseWriter.Write(ctx, invret)
			return nil
		}
		return inverr
	}
}

func (d *Dispatcher) SetResponseWriter(w flux.ServeResponseWriter) {
	d.responseWriter = w
}

func (d *Dispatcher) SetVersionLocator(l flux.WebRequestVersionLocator) {
	d.versionLocator = l
}

func (d *Dispatcher) AddOnBeforeFilterHook(h flux.OnBeforeFilterHookFunc) {
	d.onBeforeFilterHooks = append(d.onBeforeFilterHooks, h)
}

func (d *Dispatcher) AddOnBeforeTransportHook(h flux.OnBeforeTransportHookFunc) {
	d.onBeforeTransportHooks = append(d.onBeforeTransportHooks, h)
}

func (d *Dispatcher) AddOnContextHook(h flux.OnContextHookFunc) {
	d.onContextHooks = append(d.onContextHooks, h)
}

func (d *Dispatcher) selectFilters(ctx flux.Context) []flux.Filter {
	selective := make([]flux.Filter, 0, 16)
	for _, selector := range ext.FilterSelectors() {
		if selector.Activate(ctx) {
			selective = append(selective, selector.DoSelect(ctx)...)
		}
	}
	return append(ext.GlobalFilters(), selective...)
}

func (d *Dispatcher) selectPlugins(ctx flux.Context) []flux.Plugin {
	selective := make([]flux.Plugin, 0, 16)
	for _, selector := range ext.PluginSelectors() {
		if selector.Activate(ctx) {
			selective = append(selective, selector.DoSelect(ctx)...)
		}
	}
	return append(ext.GlobalPlugins(), selective...)
}

func (*Dispatcher) dup(toep *flux.EndpointSpec, fromep *flux.EndpointSpec) error {
	return copier.CopyWithOption(toep, fromep, copier.Option{
		DeepCopy: true,
	})
}
