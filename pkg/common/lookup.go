package common

import (
	"errors"
	"github.com/bytepowered/fluxgo/pkg/ext"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/toolkit"
)

// LookupValueByExpr 搜索LookupExpr表达式指定域的值。
// Expr格式： Scope:Key
func LookupValueByExpr(ctx flux.Context, expr string) (interface{}, error) {
	if expr == "" || nil == ctx {
		return nil, errors.New("empty lookup expr, or context is nil")
	}
	scope, key, ok := toolkit.ParseScopeExpr(expr)
	if !ok {
		return "", errors.New("illegal lookup expr: " + expr)
	}
	obj, err := LookupValueByScoped(ctx, scope, key)
	if nil != err {
		return "", err
	}
	return obj.Value, nil
}

// LookupValueByScoped 根据Scope,Key从Context中查找参数；支持复杂参数类型
func LookupValueByScoped(ctx flux.Context, scope, key string) (flux.EncodeValue, error) {
	if scope == "" || key == "" {
		return ext.NewNilEncodeValue(), errors.New("lookup empty scope or key, scope: " + scope + ", key: " + key)
	}
	if nil == ctx {
		return ext.NewNilEncodeValue(), errors.New("lookup nil context")
	}
	switch strings.ToUpper(scope) {
	case flux.ScopePath:
		return lookupValues(ctx.PathVars(), key), nil
	case flux.ScopePathMap:
		return ext.NewMapStringListEncodeValue(ctx.PathVars()), nil
	case flux.ScopeQuery:
		return lookupValues(ctx.QueryVars(), key), nil
	case flux.ScopeQueryMulti:
		return ext.NewListStringEncodeValue(ctx.QueryVars()[key]), nil
	case flux.ScopeQueryMap:
		return ext.NewMapStringListEncodeValue(ctx.QueryVars()), nil
	case flux.ScopeForm:
		return lookupValues(ctx.FormVars(), key), nil
	case flux.ScopeFormMap:
		return ext.NewMapStringListEncodeValue(ctx.FormVars()), nil
	case flux.ScopeFormMulti:
		return ext.NewListStringEncodeValue(ctx.FormVars()[key]), nil
	case flux.ScopeHeader:
		return lookupValues(ctx.HeaderVars(), key), nil
	case flux.ScopeHeaderMap:
		return ext.NewMapStringListEncodeValue(ctx.HeaderVars()), nil
	case flux.ScopeAttr:
		if v, ok := ctx.AttributeEx(key); ok {
			return ToEncodeValue(v), nil
		}
		return ext.NewNilEncodeValue(), nil
	case flux.ScopeAttrs:
		return ext.NewMapStringEncodeValue(ctx.Attributes()), nil
	case flux.ScopeBody:
		reader, err := ctx.BodyReader()
		if err != nil {
			return ext.NewNilEncodeValue(), err
		}
		hct := ctx.HeaderVar(flux.HeaderContentType)
		return flux.NewEncodeValue(reader, flux.EncodingType(hct)), nil
	case flux.ScopeParam:
		v, _ := LookupValues(key, ctx.QueryVars, ctx.FormVars)
		return ext.NewStringEncodeValue(v), nil
	case flux.ScopeRequest:
		switch strings.ToUpper(key) {
		case "METHOD":
			return ext.NewStringEncodeValue(ctx.Method()), nil
		case "URI":
			return ext.NewStringEncodeValue(ctx.URI()), nil
		case "HOST":
			return ext.NewStringEncodeValue(ctx.Host()), nil
		case "REMOTEADDR":
			return ext.NewStringEncodeValue(ctx.RemoteAddr()), nil
		default:
			return ext.NewNilEncodeValue(), nil
		}
	default:
		if v, ok := LookupValues(key, ctx.PathVars, ctx.QueryVars, ctx.FormVars); ok {
			return ext.NewStringEncodeValue(v), nil
		}
		if obj := lookupValues(ctx.HeaderVars(), key); obj.IsValid() {
			return obj, nil
		}
		if v, ok := ctx.AttributeEx(key); ok {
			return ToEncodeValue(v), nil
		}
		return ext.NewNilEncodeValue(), nil
	}
}

func ToEncodeValue(v interface{}) flux.EncodeValue {
	switch v.(type) {
	case int, uint, int8, uint8, int16, uint16, int32, uint32, int64, uint64, float32, float64:
		return ext.NewNumberEncodeValue(ToNumber(v))
	case string:
		return ext.NewStringEncodeValue(v.(string))
	case map[string]interface{}:
		return ext.NewMapStringEncodeValue(v.(map[string]interface{}))
	case map[string][]string:
		return ext.NewMapStringListEncodeValue(v.(map[string][]string))
	case []string:
		return ext.NewListStringEncodeValue(v.([]string))
	case []interface{}:
		return ext.NewListObjectEncodeValue(v.([]interface{}))
	default:
		return ext.NewObjectEncodeValue(v)
	}
}

func LookupValues(key string, providers ...func() url.Values) (string, bool) {
	for _, fun := range providers {
		values := fun()
		if v, ok := values[key]; ok {
			return v[0], true
		}
	}
	return "", false
}

func lookupValues(values interface{}, key string) flux.EncodeValue {
	var value []string
	var ok bool
	switch values.(type) {
	case url.Values:
		value, ok = values.(url.Values)[key]
	case http.Header:
		// Header: key case insensitive
		value, ok = values.(http.Header)[textproto.CanonicalMIMEHeaderKey(key)]
	case map[string][]string:
		value, ok = values.(map[string][]string)[key]
	default:
		ok = false
	}
	if ok {
		switch {
		case len(value) == 1:
			return ext.NewStringEncodeValue(value[0])
		case len(value) > 1:
			copied := make([]string, len(value))
			copy(copied, value)
			return ext.NewListStringEncodeValue(copied)
		default:
			return ext.NewStringEncodeValue("")
		}
	} else {
		return ext.NewNilEncodeValue()
	}
}
