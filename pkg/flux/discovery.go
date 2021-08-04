package flux

import (
	"context"
)

// MetadataDiscovery 负责对注册中心的 EndpointSpec / ServiceSpec 等元数据注册变更事件进行监听与同步。
type MetadataDiscovery interface {
	// Id 返回标识当前服务标识
	Id() string

	// SubscribeEndpoints 订阅监听Endpoint元数据配置变更事件
	SubscribeEndpoints(ctx context.Context, queue chan<- EndpointEvent) error

	// SubscribeServices 订阅监听Service元数据配置变更事件
	SubscribeServices(ctx context.Context, queue chan<- ServiceEvent) error
}

type EventType int

// 路由元数据事件类型
const (
	EventTypeAdded = iota
	EventTypeUpdated
	EventTypeRemoved
)

var eventTypeNames = map[int]string{
	-1:               "EventType:Unknown",
	EventTypeAdded:   "EventType:Added",
	EventTypeUpdated: "EventType:Updated",
	EventTypeRemoved: "EventType:Removed",
}

func EventTypeName(evtType int) string {
	if n, ok := eventTypeNames[evtType]; ok {
		return n
	}
	return eventTypeNames[-1]
}

// EndpointEvent  定义从注册中心接收到的Endpoint数据变更
type EndpointEvent struct {
	EventType EventType
	Endpoint  EndpointSpec
}

// ServiceEvent  定义从注册中心接收到的Service定义数据变更
type ServiceEvent struct {
	EventType EventType
	Service   ServiceSpec
}
