package flux

type (
	// Plugin 用于执行
	Plugin interface {
		// PluginId Plugin的标识
		PluginId() string

		// DoHandle 执行当前请求的功能
		DoHandle(ctx Context) *ServeError
	}

	// PluginSelector 用于请求处理前的动态选择Plugin
	PluginSelector interface {
		// Activate 返回当前请求是否激活Selector
		Activate(ctx Context) bool
		// DoSelect 根据请求返回激活的Plugin列表
		DoSelect(ctx Context) []Plugin
	}
)
