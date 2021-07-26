package discovery

import (
	"errors"
	"fmt"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/remoting"
)

var (
	emptyEndpoint      = flux.EndpointSpec{}
	emptyEndpointEvent = flux.EndpointEvent{}
)

func DecodeEndpoint(bytes []byte) (flux.EndpointSpec, error) {
	if err := VerifyJSON(bytes); err != nil {
		return emptyEndpoint, err
	}
	ep := flux.EndpointSpec{}
	if err := ext.JSONUnmarshal(bytes, &ep); nil != err {
		return emptyEndpoint, fmt.Errorf("DECODE:UNMARSHAL:JSON/err: %w", err)
	}
	if ep.Annotations == nil {
		ep.Annotations = make(flux.Annotations, 0)
	}
	if ep.Attributes == nil {
		ep.Attributes = make(flux.Attributes, 0)
	}
	// 检查有效性
	if !ep.IsValid() {
		return emptyEndpoint, errors.New("DECODE:VERIFY:ENDPOINT/invalid")
	}
	// 构建默认Endpoint标识
	ep.SpecKey = ext.MakeEndpointSpecKey(ep.HttpMethod, ep.HttpPattern)
	return ep, nil
}

func ToEndpointEvent(ep *flux.EndpointSpec, etype remoting.NodeEventType) (fxEvt flux.EndpointEvent, err error) {
	flux.AssertNotEmpty(ep.SpecKey, "<endpoint.SpecKey> is required")
	event := flux.EndpointEvent{Endpoint: *ep}
	switch etype {
	case remoting.EventTypeNodeAdd:
		event.EventType = flux.EventTypeAdded
	case remoting.EventTypeNodeDelete:
		event.EventType = flux.EventTypeRemoved
	case remoting.EventTypeNodeUpdate:
		event.EventType = flux.EventTypeUpdated
	default:
		return emptyEndpointEvent, fmt.Errorf("ENDPOINT:UNKNOWN_EVT_TYPE: type=%s", etype)
	}
	return event, nil
}
