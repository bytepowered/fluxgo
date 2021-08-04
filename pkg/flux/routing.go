package flux

type (
	// OnContextHookFunc 用于WebContext与Context的交互勾子；
	// 在每个请求被路由执行时，在创建Context后被调用。
	OnContextHookFunc func(WebContext, Context)

	// OnBeforeFilterHookFunc 在Filter执行前被调用的勾子函数
	OnBeforeFilterHookFunc func(Context, []Filter)

	// OnBeforeTransportHookFunc 在Transporter执行前被调用的勾子函数
	OnBeforeTransportHookFunc func(Context, Transporter)
)

// EndpointSelector 用于请求处理前的动态选择Endpoint
type EndpointSelector interface {
	// Active 判定选择器是否激活
	Active(ctx WebContext, listenerId string) bool
	// DoSelect 根据请求返回Endpoint，以及是否有效标识
	DoSelect(ctx WebContext, listenerId string, multi *MVCEndpoint) (*EndpointSpec, bool)
}
