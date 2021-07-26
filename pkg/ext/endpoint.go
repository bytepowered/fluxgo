package ext

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
	"strings"
	"sync"
)

var (
	endpoints = new(sync.Map)
)

func MakeEndpointSpecKey(method, pattern string) string {
	// Note: pattern is case-sensitive
	return strings.ToUpper(method) + "#" + pattern
}

func RegisterEndpoint(key string, endpoint *flux.EndpointSpec) *flux.MVCEndpoint {
	flux.AssertNotEmpty(key, "<key> must not empty")
	flux.AssertNotNil(endpoint, "<endpoint> must not nil")
	mvce := flux.NewMVCEndpoint(endpoint)
	endpoints.Store(key, mvce)
	return mvce
}

func EndpointByKey(key string) (*flux.MVCEndpoint, bool) {
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
