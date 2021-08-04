package flux

type (
	// FilterInvoker 定义一个处理方法，处理请求Context；
	// 如果发生错误则返回 ServeError。
	FilterInvoker func(Context) *ServeError

	// FilterSkipper 定义一个函数，用于Filter执行中跳过某些处理。
	// 返回True跳过某些处理，见具体Filter的实现逻辑。
	FilterSkipper func(Context) bool

	// Filter 用于定义处理方法的顺序及内在业务逻辑
	Filter interface {
		// FilterId Filter的类型标识
		FilterId() string
		// DoFilter 执行Filter链
		DoFilter(next FilterInvoker) FilterInvoker
	}

	// FilterSelector 用于请求处理前的动态选择Filter
	FilterSelector interface {
		// Activate 返回当前请求是否激活Selector
		Activate(Context) bool
		// DoSelect 根据请求返回激活的Filter列表
		DoSelect(Context) []Filter
	}
)
