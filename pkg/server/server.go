package server

import (
	goctx "context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"
)

import (
	dubgo "github.com/apache/dubbo-go/config"
	"golang.org/x/net/context"
	"net/http"
)

import (
	_ "net/http/pprof"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/internal"
	"github.com/bytepowered/fluxgo/pkg/logger"
	"github.com/bytepowered/fluxgo/pkg/toolkit"
)

const (
	defaultBanner = "Flux-GO // Fast gateway for microservice: dubbo, grpc, http"
	VersionFormat = "Version // git.commit=%s, build.version=%s, build.date=%s"
)

const (
	DefaultHttpHeaderVersion = "X-Version"

	ListenerIdDefault = "default"
	ListenerIdWebapi  = ListenerIdDefault
	ListenerIdAdmin   = "admin"
)

type (
	// OptionFunc 配置HttpServeEngine函数
	OptionFunc func(d *DispatchServer)
)

// DispatchServer 管理Dispatcher
type DispatchServer struct {
	dispatchers map[string]*Dispatcher
	started     chan struct{}
	stopped     chan struct{}
	banner      string
}

// WithServerBanner 配置服务Banner
func WithServerBanner(banner string) OptionFunc {
	return func(bs *DispatchServer) {
		bs.banner = banner
	}
}

// WithNewWebListener 配置WebListener
func WithNewWebListener(webListener flux.WebListener) OptionFunc {
	return func(d *DispatchServer) {
		d.AddWebListener(webListener.ListenerId(), webListener)
	}
}

// WithNewDispatcherOptions 注册WebListener的Dispatcher对象，并配置相关参数
func WithNewDispatcherOptions(webListener flux.WebListener, optfs ...DispatcherOptionFunc) OptionFunc {
	return func(d *DispatchServer) {
		d.AddWebListener(webListener.ListenerId(), webListener)
		dis := d.DispatcherById(webListener.ListenerId())
		for _, optf := range optfs {
			optf(dis)
		}
	}
}

func NewDispatcherManager(opts ...OptionFunc) *DispatchServer {
	server := &DispatchServer{
		dispatchers: make(map[string]*Dispatcher, 2),
		started:     make(chan struct{}),
		stopped:     make(chan struct{}),
		banner:      defaultBanner,
	}
	for _, opt := range opts {
		opt(server)
	}
	return server
}

// Prepare Call before init and startup
func (d *DispatchServer) Prepare() error {
	logger.Info("SERVER:EVENT:PREPARE")
	for _, hook := range ext.PrepareHooks() {
		if err := hook.OnPrepare(); nil != err {
			return err
		}
	}
	logger.Info("SERVER:EVENT:PREPARE:OK")
	return nil
}

// Init Call components init
func (d *DispatchServer) Init() error {
	logger.Info("SERVER:EVENT:INIT")
	defer logger.Info("SERVER:EVENT:INIT:OK")
	// 1. WebListen Server
	for id, dis := range d.dispatchers {
		webListener := dis.WebListener
		config := NewWebListenerConfig(id)
		ext.AddStartupHook(webListener)
		ext.AddShutdownHook(webListener)
		logger.Infow("SERVER:EVENT:INIT:WEBLISTENER", "webl-id", id, "webl-type", reflect.TypeOf(webListener))
		if err := webListener.OnInit(config); nil != err {
			return err
		}
	}
	// 2. EDS
	for _, eds := range ext.MetadataDiscoveries() {
		ext.AddStartupHook(eds)
		ext.AddShutdownHook(eds)
		err := onInitializer(eds, func(initable flux.Initializer) error {
			logger.Infow("SERVER:EVENT:INIT:DISCOVERY", "eds-id", eds.Id(), "eds-type", reflect.TypeOf(eds))
			edsc := flux.NewConfigurationByKeys(flux.NamespaceDiscoveries, eds.Id())
			return initable.OnInit(edsc)
		})
		if nil != err {
			return err
		}
	}
	// 3. Transporter
	for proto, transporter := range ext.Transporters() {
		ext.AddStartupHook(transporter)
		ext.AddShutdownHook(transporter)
		err := onInitializer(transporter, func(initable flux.Initializer) error {
			trc := flux.NewConfigurationByKeys(flux.NamespaceTransporters, proto)
			logger.Infow("SERVER:EVENT:INIT:TRANSPORT", "t-proto", proto, "t-type", reflect.TypeOf(transporter))
			return initable.OnInit(trc)
		})
		if nil != err {
			return err
		}
	}
	// 4. Static Filters
	for _, filter := range append(ext.GlobalFilters(), ext.SelectiveFilters()...) {
		err := onInitializer(filter, func(initable flux.Initializer) error {
			fic := flux.NewConfiguration(filter.FilterId())
			if IsDisabled(fic) {
				logger.Infow("SERVER:EVENT:INIT:FILTER/disabled", "f-id", filter.FilterId())
				return nil
			}
			ext.AddStartupHook(filter)
			ext.AddShutdownHook(filter)
			logger.Infow("SERVER:EVENT:INIT:FILTER", "f-id", filter.FilterId(), "f-type", reflect.TypeOf(filter))
			return initable.OnInit(fic)
		})
		if nil != err {
			return err
		}
	}
	// 5. Dynamic Filters
	dynFilters, err := dynamicFilters()
	if nil != err {
		return err
	}
	for _, dynf := range dynFilters {
		inst := dynf.Factory()
		dynfilter, ok := inst.(flux.Filter)
		var msgs = []interface{}{"dynf-id", dynf.Id, "dynf-type-id", dynf.TypeId, "dynf-type", reflect.TypeOf(inst)}
		if !ok {
			logger.Infow("SERVER:EVENT:INIT:DYNFILTER/malformed", msgs...)
			continue
		}
		logger.Infow("SERVER:EVENT:INIT:DYNFILTER/new", msgs)
		if IsDisabled(dynf.Config) {
			logger.Infow("SERVER:EVENT:INIT:DYNFILTER/disable", "dynf-id", dynf.Id)
			continue
		}
		ext.AddSelectiveFilter(dynfilter)
		ext.AddStartupHook(dynfilter)
		ext.AddShutdownHook(dynfilter)
		err := onInitializer(dynfilter, func(initable flux.Initializer) error {
			logger.Infow("SERVER:EVENT:INIT:FILTER", "dynf-id", dynfilter.FilterId())
			return initable.OnInit(dynf.Config)
		})
		if nil != err {
			return err
		}
	}
	return nil
}

func (d *DispatchServer) Startup(build flux.Build) error {
	fmt.Printf(VersionFormat, build.CommitId, build.Version, build.Date)
	if d.banner != "" {
		fmt.Println(d.banner)
	}
	return d.start()
}

func (d *DispatchServer) start() error {
	flux.AssertNotNil(d.defaultListener(), "<default-listener> MUST NOT nil")
	flux.AssertNotNil(ext.GetLookupScopedValueFunc(), "<scope-value-lookup-func> MUST NOT nil")
	logger.Info("SERVER:EVENT:STARTUP")
	for _, startup := range sortedStartup(ext.StartupHooks()) {
		if err := startup.OnStartup(); nil != err {
			return err
		}
	}
	logger.Info("SERVER:EVENT:STARTUP:OK")
	// Discovery
	endpoints := make(chan flux.EndpointEvent, 2)
	services := make(chan flux.ServiceEvent, 2)
	defer func() {
		close(endpoints)
		close(services)
	}()
	logger.Info("SERVER:EVENT:DISCOVERY:START")
	ctx, canceled := context.WithCancel(context.Background())
	defer canceled()
	go d.startEventLoop(ctx, endpoints, services)
	if err := d.startEventWatch(ctx, endpoints, services); nil != err {
		return err
	}
	logger.Info("SERVER:EVENT:DISCOVERY:OK")
	// Listeners
	errch := make(chan error, 1)
	for id, dis := range d.dispatchers {
		logger.Infow("SERVER:EVENT:LISTENER:START", "listener-id", id)
		go func(id string, listener flux.WebListener) {
			errch <- listener.ListenServe()
			logger.Infow("SERVER:EVENT:LISTENER:STOP", "listener-id", id)
		}(id, dis.WebListener)
	}
	close(d.started)
	return <-errch
}

func (d *DispatchServer) startEventLoop(ctx context.Context, endpoints chan flux.EndpointEvent, services chan flux.ServiceEvent) {
	logger.Info("SERVER:EVENT:EVENTLOOP:START")
	defer logger.Info("SERVER:EVENT:EVENTLOOP:STOP")
	for {
		select {
		case epEvt, ok := <-endpoints:
			if ok {
				d.onEndpointEvent(epEvt)
			}

		case esEvt, ok := <-services:
			if ok {
				d.onServiceEvent(esEvt)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (d *DispatchServer) startEventWatch(ctx context.Context, endpoints chan flux.EndpointEvent, services chan flux.ServiceEvent) error {
	for _, discovery := range ext.MetadataDiscoveries() {
		logger.Infow("SERVER:EVENT:DISCOVERY:WATCH", "discovery-id", discovery.Id())
		if err := discovery.SubscribeServices(ctx, services); nil != err {
			return err
		}
		if err := discovery.SubscribeEndpoints(ctx, endpoints); nil != err {
			return err
		}
		logger.Infow("SERVER:EVENT:DISCOVERY:WATCH/OK", "discovery-id", discovery.Id())
	}
	return nil
}

// Shutdown to cleanup resources
func (d *DispatchServer) Shutdown(ctx goctx.Context) error {
	logger.Info("SERVER:EVENT:SHUTDOWN")
	defer func() {
		logger.Info("SERVER:EVENT:SHUTDOWN/ok")
		close(d.stopped)
	}()
	// WebListener
	for id, dis := range d.dispatchers {
		if err := dis.WebListener.OnShutdown(ctx); nil != err {
			logger.Warnw("Server["+id+"] shutdown http server", "error", err)
		}
	}
	// Components
	for _, shutdown := range sortedShutdown(ext.ShutdownHooks()) {
		if err := shutdown.OnShutdown(ctx); nil != err {
			logger.Warnw("Component shutdown failed", "c-type", reflect.TypeOf(shutdown), "error", err)
		}
	}
	return nil
}

func (d *DispatchServer) serve(webex flux.WebContext, versions *flux.MVCEndpoint) (err error) {
	dis := d.DispatcherById(webex.WebListener().ListenerId())
	return dis.route(webex, versions)
}

func (d *DispatchServer) DispatcherById(id string) *Dispatcher {
	dis := d.dispatchers[id]
	flux.AssertNotNil(dis, "<dispatcher> must not nil, id: "+id)
	return dis
}

func (d *DispatchServer) onServiceEvent(event flux.ServiceEvent) {
	service := event.Service
	var epvars = []interface{}{"service-id", service.ServiceID(), "alias-id", service.AliasId}
	if err := internal.VerifyAnnotations(service.Annotations); err != nil {
		logger.Warnw("SERVER:EVENT:SERVICE:ANNOTATION/invalid", epvars...)
		return
	}
	switch event.EventType {
	case flux.EventTypeAdded:
		logger.Infow("SERVER:EVENT:SERVICE:ADD", epvars...)
		d.syncEndpoint(&service)
		ext.RegisterService(service)
		if service.AliasId != "" {
			ext.RegisterServiceByID(service.AliasId, service)
		}

	case flux.EventTypeUpdated:
		logger.Infow("SERVER:EVENT:SERVICE:UPDATE", epvars...)
		d.syncEndpoint(&service)
		ext.RegisterService(service)
		if service.AliasId != "" {
			ext.RegisterServiceByID(service.AliasId, service)
		}
	case flux.EventTypeRemoved:
		logger.Infow("SERVER:EVENT:SERVICE:REMOVE", epvars...)
		ext.RemoveServiceByID(service.ServiceID())
		if service.AliasId != "" {
			ext.RemoveServiceByID(service.AliasId)
		}
	}
}

func (d *DispatchServer) onEndpointEvent(event flux.EndpointEvent) {
	ep := event.Endpoint
	var epvars = []interface{}{"ep-app", ep.Application, "ep-version", ep.Version, "ep-method", ep.HttpMethod, "ep-pattern", ep.HttpPattern}
	// Check http method
	method := strings.ToUpper(event.Endpoint.HttpMethod)
	if !SupportedHttpMethod(method) {
		logger.Warnw("SERVER:EVENT:ENDPOINT:METHOD/ignore", epvars...)
		return
	}
	if err := internal.VerifyAnnotations(ep.Annotations); err != nil {
		logger.Warnw("SERVER:EVENT:ENDPOINT:ANNOTATION/invalid", epvars...)
		return
	}
	mvce, register := d.selectMVCEndpoint(&ep)
	switch event.EventType {
	case flux.EventTypeAdded:
		logger.Infow("SERVER:EVENT:ENDPOINT:ADD", epvars...)
		d.syncService(&ep)
		mvce.Update(ep.Version, &ep)
		if register {
			// 根据Endpoint注解属性，选择ListenServer来绑定
			var listenerId = ListenerIdDefault
			if anno, ok := ep.AnnotationEx(flux.EndpointAnnotationListenerSel); ok && anno.IsValid() {
				listenerId = anno.GetString()
			}
			if webListener, ok := d.WebListenerById(listenerId); ok {
				logger.Infow("SERVER:EVENT:ENDPOINT:HTTP_HANDLER/"+listenerId, epvars...)
				webListener.AddHandler(ep.HttpMethod, ep.HttpPattern, d.newEndpointHandler(mvce))
			} else {
				logger.Errorw("SERVER:EVENT:ENDPOINT:LISTENER_MISSED/"+listenerId, epvars...)
			}
		}
	case flux.EventTypeUpdated:
		logger.Infow("SERVER:EVENT:ENDPOINT:UPDATE", epvars...)
		d.syncService(&ep)
		mvce.Update(ep.Version, &ep)
	case flux.EventTypeRemoved:
		logger.Infow("SERVER:EVENT:ENDPOINT:REMOVE", epvars...)
		mvce.Delete(ep.Version)
	}
}

// AwaitSignal GracefulShutdown
func (d *DispatchServer) AwaitSignal(quit chan os.Signal, to time.Duration) {
	// 接收停止信号
	signal.Notify(quit, dubgo.ShutdownSignals...)
	<-quit
	logger.Infof("SERVER:EVENT:SIGNAL:SHUTDOWN")
	ctx, cancel := goctx.WithTimeout(goctx.Background(), to)
	defer cancel()
	if err := d.Shutdown(ctx); nil != err {
		logger.Errorw("SERVER:EVENT:SHUTDOWN/error", "error", err)
	}
}

// StateStarted 返回一个Channel。当服务启动完成时，此Channel将被关闭。
func (d *DispatchServer) StateStarted() <-chan struct{} {
	return d.started
}

// StateStopped 返回一个Channel。当服务停止后完成时，此Channel将被关闭。
func (d *DispatchServer) StateStopped() <-chan struct{} {
	return d.stopped
}

// AddWebFilter 添加Http前拦截器到默认ListenerServer。将在Http被路由到对应Handler之前执行
func (d *DispatchServer) AddWebFilter(m flux.WebFilter) {
	d.defaultListener().AddFilter(m)
}

// AddWebHandler 添加Http处理接口到默认ListenerServer。
func (d *DispatchServer) AddWebHandler(method, pattern string, h flux.WebHandlerFunc, m ...flux.WebFilter) {
	d.defaultListener().AddHandler(method, pattern, h, m...)
}

// AddWebHttpHandler 添加Http处理接口到默认ListenerServer。
func (d *DispatchServer) AddWebHttpHandler(method, pattern string, h http.Handler, m ...func(http.Handler) http.Handler) {
	d.defaultListener().AddHttpHandler(method, pattern, h, m...)
}

// SetWebNotfoundHandler 设置Http路由失败的处理接口到默认ListenerServer
func (d *DispatchServer) SetWebNotfoundHandler(nfh flux.WebHandlerFunc) {
	d.defaultListener().SetNotfoundHandler(nfh)
}

// AddWebListener 添加指定ID
func (d *DispatchServer) AddWebListener(listenerID string, listener flux.WebListener) {
	flux.AssertNotNil(listener, "<web-listener> must not nil")
	flux.AssertNotEmpty(listenerID, "<web-listener-id> must not empty")
	d.dispatchers[listenerID] = newDispatcher(listener)
}

// WebListenerById 返回ListenServer实例
func (d *DispatchServer) WebListenerById(listenerID string) (flux.WebListener, bool) {
	flux.AssertNotEmpty(listenerID, "<web-listener-id> must not empty")
	dis, ok := d.dispatchers[listenerID]
	return dis.WebListener, ok
}

func (d *DispatchServer) newEndpointHandler(endpoint *flux.MVCEndpoint) flux.WebHandlerFunc {
	return func(webex flux.WebContext) error {
		return d.serve(webex, endpoint)
	}
}

func (d *DispatchServer) selectMVCEndpoint(endpoint *flux.EndpointSpec) (*flux.MVCEndpoint, bool) {
	key := ext.MakeEndpointKey(endpoint.HttpMethod, endpoint.HttpPattern)
	if mve, ok := ext.EndpointByKey(key); ok {
		return mve, false
	} else {
		return ext.RegisterEndpoint(key, endpoint), true
	}
}

func (d *DispatchServer) defaultListener() flux.WebListener {
	count := len(d.dispatchers)
	if count == 0 {
		return nil
	} else if count == 1 {
		for _, dis := range d.dispatchers {
			return dis.WebListener
		}
	}
	dis, ok := d.dispatchers[ListenerIdDefault]
	if ok {
		return dis.WebListener
	}
	return nil
}

// syncService 将Endpoint与Service建立绑定映射；
// 此处绑定的为原始元数据的引用；
func (d *DispatchServer) syncService(ep *flux.EndpointSpec) {
	// Endpoint为静态模型，不支持动态更新
	if ep.AnnotationExists(flux.EndpointAnnotationStaticModel) {
		logger.Infow("SERVER:EVENT:SYN-MODEL/ignore:static", "ep-pattern", ep.HttpPattern, "ep-service", ep.ServiceId)
		return
	}
	service, ok := ext.ServiceByID(ep.ServiceId)
	if !ok {
		return
	}
	logger.Infow("SERVER:EVENT:SYN-MODEL/sync-service", "ep-pattern", ep.HttpPattern, "ep-service", ep.ServiceId)
	ep.Service = service
}

// syncEndpoint 将Endpoint与Service建立绑定映射；
// 此处绑定的为原始元数据的引用；
func (d *DispatchServer) syncEndpoint(srv *flux.ServiceSpec) {
	for _, mvce := range ext.Endpoints() {
		for _, ep := range mvce.Endpoints() {
			// Endpoint为静态模型，不支持动态更新
			if ep.AnnotationExists(flux.EndpointAnnotationStaticModel) {
				logger.Infow("SERVER:EVENT:SYN-MODEL/ignore:static", "ep-pattern", ep.HttpPattern, "ep-service", ep.ServiceId)
				continue
			}
			if toolkit.MatchEqual([]string{srv.ServiceID(), srv.AliasId}, ep.ServiceId) {
				logger.Infow("SERVER:EVENT:SYN-MODEL/sync-endpoint", "ep-pattern", ep.HttpPattern, "ep-service", ep.ServiceId)
				ep.Service = *srv
			}
		}
	}
}

func onInitializer(v interface{}, f func(initable flux.Initializer) error) error {
	if init, ok := v.(flux.Initializer); ok {
		return f(init)
	}
	return nil
}

// NewWebListenerConfig 根据WebListenerID，返回初始化WebListener实例时的配置
func NewWebListenerConfig(id string) *flux.Configuration {
	return flux.NewConfigurationByKeys(flux.NamespaceWebListeners, id)
}

// NewWebListenerOptions 根据WebListenerID，返回构建WebListener实例时的选项参数
func NewWebListenerOptions(id string) *flux.Configuration {
	return flux.NewConfigurationByKeys(flux.NamespaceWebListeners, id)
}

func SupportedHttpMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut,
		http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodTrace:
		return true
	default:
		logger.Errorw("Ignore unsupported http method:", "method", method)
		return false
	}
}
