package flux

import (
	"strings"
	"sync"
)

// Endpoint内置注解
const (
	EndpointAnnotationPermissions = "flux.go/permissions"       // 权限权限声明
	EndpointAnnotationBizKey      = "flux.go/biz.key"           // 标识Endpoint绑定到业务标识
	EndpointAnnotationAuthorize   = "flux.go/authorize"         // 标识Endpoint访问是否需要授权的注解
	EndpointAnnotationListenerSel = "flux.go/listener.selector" // 标识Endpoint绑定到哪个ListenServer服务
	EndpointAnnotationStaticModel = "flux.go/static.model"      // 标识此Endpoint为固定数据模型，不支持动态更新
)

// EndpointSpec 定义前端Http请求与后端RPC服务的端点元数据
type EndpointSpec struct {
	SpecKey     string      `json:"-"`                              // 标识Endpoint端点的Key
	Kind        string      `json:"kind" yaml:"kind"`               // Endpoint类型
	Application string      `json:"application" yaml:"application"` // 所属应用名
	Version     string      `json:"version" yaml:"version"`         // 端点版本号
	HttpPattern string      `json:"httpPattern" yaml:"httpPattern"` // 映射Http侧的UriPattern
	HttpMethod  string      `json:"httpMethod" yaml:"httpMethod"`   // 映射Http侧的Method
	Attributes  Attributes  `json:"attributes" yaml:"attributes"`   // 属性列表
	Annotations Annotations `json:"annotations" yaml:"annotations"` // 注解列表
	ServiceId   string      `json:"serviceId" yaml:"serviceId"`     // 上游/后端服务ServiceId
	Service     ServiceSpec `json:"service"`                        // 上游/后端服务
}

// IsValid 判断Endpoint配置是否有效；
// - HttpMethod, HttpPattern, ServiceId 不能为空；
// - 字段 Attributes, Annotation 非Nil；
func (e *EndpointSpec) IsValid() bool {
	return e.HttpMethod != "" && e.HttpPattern != "" && e.ServiceId != "" &&
		e.Attributes != nil && e.Annotations != nil
}

// Attribute 获取指定名称的属性；如果属性不存在，返回空属性对象。
func (e *EndpointSpec) Attribute(name string) NamedValueSpec {
	return e.Attributes.Single(name)
}

// AttributeEx 获取指定名称的属性，如果属性不存在，返回空属性；并返回属性是否有效的标识；
func (e *EndpointSpec) AttributeEx(name string) (NamedValueSpec, bool) {
	return e.Attributes.SingleEx(name)
}

// AttributeExists 判断指定名称的属性是否存在
func (e *EndpointSpec) AttributeExists(name string) bool {
	return e.Attributes.Exists(name)
}

// MultiAttributes 获取指定属性名的多个属性列表
func (e *EndpointSpec) MultiAttributes(name string) Attributes {
	return e.Attributes.Multiple(name)
}

// Annotation 获取指定名称的注解，如果注解不存在，返回空注解。
func (e *EndpointSpec) Annotation(name string) NamedValueSpec {
	return e.Annotations.Get(name)
}

// AnnotationEx 获取指定名称的注解，如果注解不存在，返回空注解。并返回注解是否有效的标识；
func (e *EndpointSpec) AnnotationEx(name string) (NamedValueSpec, bool) {
	return e.Annotations.GetEx(name)
}

// AnnotationExists 判断指定名称的注解是否存在
func (e *EndpointSpec) AnnotationExists(name string) bool {
	return e.Annotations.Exists(name)
}

// Attributes 定义属性列表
type Attributes []NamedValueSpec

// Single 查询单个属性
func (a Attributes) Single(name string) NamedValueSpec {
	v, _ := a.SingleEx(name)
	return v
}

// SingleEx 查询单个属性，并返回属性是否有效的标识
func (a Attributes) SingleEx(name string) (NamedValueSpec, bool) {
	for _, attr := range a {
		if strings.EqualFold(attr.Name, name) {
			return attr, true
		}
	}
	return NamedValueSpec{}, false
}

// Multiple 查询多个相同名称的属性，返回新的属性列表
func (a Attributes) Multiple(name string) Attributes {
	out := make(Attributes, 0, 2)
	for _, attr := range a {
		if strings.EqualFold(attr.Name, name) {
			out = append(out, attr)
		}
	}
	return out
}

// Exists 判定属性名是否存在
func (a Attributes) Exists(name string) bool {
	for _, attr := range a {
		if strings.EqualFold(attr.Name, name) {
			return true
		}
	}
	return false
}

// GetStrings 返回属性列表的字符串值
func (a Attributes) GetStrings() []string {
	out := make([]string, len(a))
	for i, a := range a {
		out[i] = a.GetString()
	}
	return out
}

// Append 追加新的属性，并返回新的属性列表
func (a Attributes) Append(in NamedValueSpec) Attributes {
	return append(a, in)
}

// MVCEndpoint 维护多个版本号的Endpoint对象
type MVCEndpoint struct {
	versions map[string]*EndpointSpec // 各版本数据
	*sync.RWMutex
}

func NewMVCEndpoint(endpoint *EndpointSpec) *MVCEndpoint {
	return &MVCEndpoint{
		versions: map[string]*EndpointSpec{
			endpoint.Version: endpoint,
		},
		RWMutex: new(sync.RWMutex),
	}
}

// IsEmpty 判断是否为空
func (m *MVCEndpoint) IsEmpty() bool {
	m.RLock()
	defer m.RUnlock()
	return len(m.versions) == 0
}

// Lookup 按指定版本号查找匹配的Endpoint；注意返回值是数据引用指针；
// 如果有且仅有一个版本，则直接返回此Endpoint，不比较版本号是否匹配。
func (m *MVCEndpoint) Lookup(version string) (*EndpointSpec, bool) {
	m.RLock()
	defer m.RUnlock()
	size := len(m.versions)
	if 0 == size {
		return nil, false
	}
	if "" == version || 1 == size {
		for _, epv := range m.versions {
			return epv, true
		}
	}
	epv, ok := m.versions[version]
	if !ok {
		return nil, false
	}
	return epv, true
}

// Update 更新指定版本号的Endpoint元数据
func (m *MVCEndpoint) Update(version string, endpoint *EndpointSpec) {
	m.Lock()
	m.versions[version] = endpoint
	m.Unlock()
}

// Delete 删除指定版本号的Endpoint元数据
func (m *MVCEndpoint) Delete(version string) {
	m.Lock()
	delete(m.versions, version)
	m.Unlock()
}

// Random 随机读取一个版本的Endpoint元数据。
// 注意：必须保证随机读取版本时存在非空元数据，否则会报错panic。
func (m *MVCEndpoint) Random() EndpointSpec {
	m.RLock()
	defer m.RUnlock()
	for _, ep := range m.versions {
		return *ep
	}
	panic("SERVER:CRITICAL:ASSERT: <multi-endpoint> must not empty, call by random query func")
}

// Endpoints 获取当前多版本控制器的全部Endpoint元数据列表
func (m *MVCEndpoint) Endpoints() []*EndpointSpec {
	m.RLock()
	copies := make([]*EndpointSpec, 0, len(m.versions))
	for _, ep := range m.versions {
		copies = append(copies, ep)
	}
	m.RUnlock()
	return copies
}
