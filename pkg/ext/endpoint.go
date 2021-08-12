package ext

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
	"regexp"
	"strings"
	"sync"
)

var (
	endpoints = new(sync.Map)
	epregexp  = regexp.MustCompile(`\{[^}]*\}`)
)

// MakeEndpointSpecKey 构建HttpEndpoint的唯一标识端点Key。
// 注意1：Http Pattern 大小写敏感；
// 注意2：Restful的动态路径，以其模式作为唯一标识；
func MakeEndpointSpecKey(method, pattern string) string {
	// Note: pattern is case-sensitive
	// Note: pattern must be unique, dynamic path
	method = strings.ToUpper(method)
	if !strings.Contains(pattern, "{") {
		return method + "#" + pattern
	} else {
		return method + "#" + epregexp.ReplaceAllString(pattern, "+")
	}
}

func RegisterEndpoint(key string, endpoint *flux.EndpointSpec) *flux.MVCEndpoint {
	flux.AssertNotEmpty(key, "<key> must not empty")
	flux.AssertNotNil(endpoint, "<endpoint> must not nil")
	mvce := flux.NewMVCEndpoint(endpoint)
	endpoints.Store(key, mvce)
	return mvce
}

func EndpointBySpecKey(key string) (*flux.MVCEndpoint, bool) {
	ep, ok := endpoints.Load(key)
	if ok {
		return ep.(*flux.MVCEndpoint), true
	}
	return nil, false
}

func Endpoints() map[string]*flux.MVCEndpoint {
	out := make(map[string]*flux.MVCEndpoint, 128)
	endpoints.Range(func(key, value interface{}) bool {
		out[key.(string)] = value.(*flux.MVCEndpoint)
		return true
	})
	return out
}
