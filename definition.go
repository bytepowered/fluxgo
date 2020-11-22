package flux

import "fmt"

type (
	EventType int
)

// 路由元数据事件类型
const (
	EndpointEventAdded = iota
	EndpointEventUpdated
	EndpointEventRemoved
)

const (
	// 从动态Path参数中获取
	ScopePath = "PATH"
	// 从Query参数中获取
	ScopeQuery = "QUERY"
	// 只从Form表单参数参数列表中读取
	ScopeForm = "FORM"
	// 只从Query和Form表单参数参数列表中读取
	ScopeParam = "PARAM"
	// 只从Header参数中读取
	ScopeHeader = "HEADER"
	// 获取Http Attributes的单个参数
	ScopeAttr = "ATTR"
	// 获取Http Attributes的Map结果
	ScopeAttrs = "ATTRS"
	// 获取Body数据
	ScopeBody = "BODY"
	// 获取Request元数据
	ScopeRequest = "REQUEST"
	// 自动查找数据源
	ScopeAuto = "AUTO"
)

const (
	// 原始参数类型：int,long...
	ArgumentTypePrimitive = "PRIMITIVE"
	// 复杂参数类型：POJO
	ArgumentTypeComplex = "COMPLEX"
)

// Support protocols
const (
	ProtoDubbo = "DUBBO"
	ProtoGRPC  = "GRPC"
	ProtoHttp  = "HTTP"
)

// ArgumentValueLookupFunc 参数值查找函数
type ArgumentValueLookupFunc func(scope, key string, context Context) (value MIMEValue, err error)

// ArgumentValueResolveFunc 参数值解析函数
type ArgumentValueResolveFunc func(mimeValue MIMEValue, argument Argument, context Context) (value interface{}, err error)

// Argument 定义Endpoint的参数结构元数据
type Argument struct {
	Name      string     `json:"name"`      // 参数名称
	Type      string     `json:"type"`      // 参数结构类型
	Class     string     `json:"class"`     // 参数类型
	Generic   []string   `json:"generic"`   // 泛型类型
	HttpName  string     `json:"httpName"`  // 映射Http的参数Key
	HttpScope string     `json:"httpScope"` // 映射Http参数值域
	Fields    []Argument `json:"fields"`    // 子结构字段
}

// BackendService 定义连接上游目标服务的信息
type BackendService struct {
	RemoteHost string     `json:"remoteHost"` // Service侧的Host
	Interface  string     `json:"interface"`  // Service侧的URL
	Method     string     `json:"method"`     // Service侧的方法
	Arguments  []Argument `json:"arguments"`  // Service侧的参数结构
	RpcProto   string     `json:"rpcProto"`   // Service侧的协议
	RpcGroup   string     `json:"rpcGroup"`   // Service侧的接口分组
	RpcVersion string     `json:"rpcVersion"` // Service侧的接口版本
	RpcTimeout string     `json:"rpcTimeout"` // Service侧的调用超时
	RpcRetries string     `json:"rpcRetries"` // Service侧的调用重试
}

// Endpoint 定义前端Http请求与后端RPC服务的端点元数据
type Endpoint struct {
	Application string                 `json:"application"` // 所属应用名
	Version     string                 `json:"version"`     // 端点版本号
	HttpPattern string                 `json:"httpPattern"` // 映射Http侧的UriPattern
	HttpMethod  string                 `json:"httpMethod"`  // 映射Http侧的Method
	Authorize   bool                   `json:"authorize"`   // 此端点是否需要授权
	Service     BackendService         `json:"service"`     // 上游服务
	Permission  PermissionService      `json:"permission"`  // 权限验证定义
	Extensions  map[string]interface{} `json:"extensions"`  // 扩展信息
}

// PermissionService 后端RPC服务的权限验证的元数据
type PermissionService BackendService

func (p PermissionService) IsValid() bool {
	return "" != p.RpcProto && "" != p.Interface && "" != p.Method && len(p.Arguments) > 0
}

// NewServiceKey 构建标识一个Service的Key字符串
func NewServiceKey(proto, host, method, uri string) string {
	return fmt.Sprintf("%s@%s:%s/%s", proto, host, method, uri)
}

// EndpointEvent  定义从注册中心接收到的Endpoint数据变更
type EndpointEvent struct {
	EventType   EventType
	HttpMethod  string `json:"method"`
	HttpPattern string `json:"pattern"`
	Endpoint    Endpoint
}
