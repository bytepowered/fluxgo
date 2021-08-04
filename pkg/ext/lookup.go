package ext

import "github.com/bytepowered/fluxgo/pkg/flux"

// LookupScopedValueFunc 参数值查找函数
type LookupScopedValueFunc func(ctx flux.Context, scope, key string) (flux.EncodeValue, error)

// 提供一种可扩展的参数查找实现。
// 通过替换参数值查找函数，可以允许某些非规范Http参数系统的自定义参数值查找逻辑。
var (
	lookupScopedValueFunc LookupScopedValueFunc
)

func SetLookupScopedValueFunc(f LookupScopedValueFunc) {
	lookupScopedValueFunc = flux.MustNotNil(f,
		"<lookup-scoped-value-func> must not nil").(LookupScopedValueFunc)
}

func GetLookupScopedValueFunc() LookupScopedValueFunc {
	return lookupScopedValueFunc
}
