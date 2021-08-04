package flux

import (
	"time"
)

const (
	XRequestId   = "X-Request-Id"
	XRequestTime = "X-Request-Time"
)

// Context 定义每个请求的上下文环境。包含当前请求的Endpoint、属性Attributes、请求Context等。
// Context是一个与请求生命周期相同的临时上下文容器，它在Http请求被接受处理时创建，并在请求被处理完成时销毁回收。
type Context interface {
	WebContext

	// Application 返回当前Endpoint对应的应用名
	Application() string

	// Endpoint 返回当前请求路由定义的Endpoint元数据
	Endpoint() *EndpointSpec

	// Exposed 返回当前绑定暴露到Http服务端口的三元组
	Exposed() (pattern, method, version string)

	// Service 返回Service信息
	Service() ServiceSpec

	// ServiceID 返回Endpoint Service的服务标识
	ServiceID() string

	// Attribute 获取指定key的Attribute。如果不存在，返回默认值；
	// Attr搜索位置及优先级顺序：
	// 1. Context自身的Attributes；
	// 2. Endpoint的Attributes；
	Attribute(key string, defval interface{}) interface{}

	// Attributes 返回所有Attributes键值对；只读；
	Attributes() map[string]interface{}

	// AttributeEx 获取指定key的Attribute，返回值和是否存在标识
	// Attr搜索位置及优先级顺序：
	// 1. Context自身的Attributes；
	// 2. Endpoint的Attributes；
	AttributeEx(key string) (interface{}, bool)

	// SetAttribute 向Context添加Attribute键值对
	SetAttribute(key string, value interface{})

	// StartAt 返回Http请求起始的服务器时间
	StartAt() time.Time

	// AddMetric 添加路由耗时统计节点
	AddMetric(name string, elapsed time.Duration)

	// Metrics 返回请求路由的的统计数据
	Metrics() []TraceMetric

	// SetLogger 添加Context范围的Logger。
	// 通常是将关联一些追踪字段的Logger设置为ContextLogger
	SetLogger(logger Logger)

	// Logger 返回Context范围的Logger。
	Logger() Logger
}

// TraceMetric 请求路由的的统计数据
type TraceMetric struct {
	Name    string `json:"name"`
	Latency string `json:"latency"`
}

// Logger 日志Logger接口定义
type Logger interface {
	Info(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Debug(args ...interface{})
	Panic(args ...interface{})
	Infof(fmt string, args ...interface{})
	Warnf(fmt string, args ...interface{})
	Errorf(fmt string, args ...interface{})
	Debugf(fmt string, args ...interface{})
	Panicf(fmt string, args ...interface{})
	Infow(msg string, keyAndValues ...interface{})
	Warnw(msg string, keyAndValues ...interface{})
	Errorw(msg string, keyAndValues ...interface{})
	Debugw(msg string, keyAndValues ...interface{})
	Panicw(msg string, keyAndValues ...interface{})
}
