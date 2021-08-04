package internal

import (
	"go.uber.org/zap"
	"time"
)

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
)

// ContextKey 用于隐藏内部实现的Key
type ContextKey string

var (
	CtxkeyRequestId  = ContextKey("flux.go/context.request.id/926820fa-7ad8-4444-9080-d690ce31c93a")
	CtxkeyWebContext = ContextKey("flux.go/context.web.context/890b1fa9-93ad-4b44-af24-85bcbfe646b4")
)

var _ flux.Context = new(Context)

// Context 定义每个请求的上下文环境。包含当前请求的Endpoint、属性Attributes、请求Context等。
// Context是一个与请求生命周期相同的临时上下文容器，它在Http请求被接受处理时创建，并在请求被处理完成时销毁回收。
type Context struct {
	flux.WebContext
	endpoint   *flux.EndpointSpec
	attributes map[string]interface{}
	metrics    []flux.TraceMetric
	startTime  time.Time
	ctxLogger  flux.Logger
}

// NewContext 构建新的Context实例。
func NewContext() *Context {
	return &Context{
		attributes: make(map[string]interface{}, 16),
		metrics:    make([]flux.TraceMetric, 0, 16),
	}
}

// Reset 重置Context，重新与 WebContext 关联，绑定新的 EndpointSpec
// Note：此函数由内部框架调用，不作为外部使用。
func (c *Context) Reset(webex flux.WebContext, endpoint *flux.EndpointSpec) {
	c.WebContext = webex
	c.endpoint = endpoint
	c.ctxLogger = zap.S()
	c.startTime = time.Now()
	c.metrics = c.metrics[:0]
	for k := range c.attributes {
		delete(c.attributes, k)
	}
}

// Application 返回当前Endpoint对应的应用名
func (c *Context) Application() string {
	return c.endpoint.Application
}

// Endpoint 返回当前请求路由定义的Endpoint元数据
func (c *Context) Endpoint() *flux.EndpointSpec {
	return c.endpoint
}

// Exposed 返回当前绑定暴露到Http服务端口的三元组
func (c *Context) Exposed() (pattern, method, version string) {
	return c.endpoint.HttpPattern, c.endpoint.HttpMethod, c.endpoint.Version
}

// Service 返回Service信息
func (c *Context) Service() flux.ServiceSpec {
	return c.endpoint.Service
}

// ServiceID 返回Endpoint Service的服务标识
func (c *Context) ServiceID() string {
	return c.endpoint.Service.ServiceID()
}

// Attribute 获取指定key的Attribute。如果不存在，返回默认值；
func (c *Context) Attribute(key string, defval interface{}) interface{} {
	if v, ok := c.AttributeEx(key); ok {
		return v
	} else {
		return defval
	}
}

// Attributes 返回所有Attributes键值对；只读；
// 搜索位置及顺序：
// 1. 【HIGH】Context自身的Attributes；
// 2. 【LOW】Endpoint的Attributes；
func (c *Context) Attributes() map[string]interface{} {
	out := make(map[string]interface{}, len(c.attributes))
	for _, attr := range c.endpoint.Attributes {
		out[attr.Name] = attr.Value
	}
	for k, v := range c.attributes {
		out[k] = v
	}
	return out
}

// AttributeEx 获取指定key的Attribute，返回值和是否存在标识
// 搜索位置及顺序：
// 1. Context自身的Attributes；【HIGH】
// 2. Endpoint的Attributes；【LOW】
func (c *Context) AttributeEx(key string) (interface{}, bool) {
	v, ok := c.attributes[key]
	if !ok {
		if attr, aok := c.endpoint.AttributeEx(key); aok {
			return attr.Value, true
		}
	}
	return v, ok
}

// SetAttribute 向Context添加Attribute键值对
func (c *Context) SetAttribute(key string, value interface{}) {
	c.attributes[key] = value
}

// StartAt 返回Http请求起始的服务器时间
func (c *Context) StartAt() time.Time {
	return c.startTime
}

// AddMetric 添加路由耗时统计节点
func (c *Context) AddMetric(name string, elapsed time.Duration) {
	c.metrics = append(c.metrics, flux.TraceMetric{
		Name: name, Latency: elapsed.String(),
	})
}

// Metrics 返回请求路由的的统计数据
func (c *Context) Metrics() []flux.TraceMetric {
	dist := make([]flux.TraceMetric, len(c.metrics))
	copy(dist, c.metrics)
	return dist
}

// SetLogger 添加Context范围的Logger。
// 通常是将关联一些追踪字段的Logger设置为ContextLogger
func (c *Context) SetLogger(logger flux.Logger) {
	c.ctxLogger = logger
}

// Logger 返回Context范围的Logger。
func (c *Context) Logger() flux.Logger {
	return c.ctxLogger
}
