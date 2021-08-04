package flux

import (
	"github.com/spf13/cast"
	"reflect"
	"strings"
)

const (
	// ScopePath 从动态Path参数中获取
	ScopePath = "PATH"
	// ScopePathMap 查询所有Path参数
	ScopePathMap = "PATH_MAP"
	// ScopeQuery 从Query参数中获取
	ScopeQuery      = "QUERY"
	ScopeQueryMulti = "QUERY_MUL"
	// ScopeQueryMap 获取全部Query参数
	ScopeQueryMap = "QUERY_MAP"
	// ScopeForm 只从Form表单参数参数列表中读取
	ScopeForm      = "FORM"
	ScopeFormMulti = "FORM_MUL"
	// ScopeFormMap 获取Form全部参数
	ScopeFormMap = "FORM_MAP"
	// ScopeParam 只从Query和Form表单参数参数列表中读取
	ScopeParam = "PARAM"
	// ScopeHeader 只从Header参数中读取
	ScopeHeader = "HEADER"
	// ScopeHeaderMap 获取Header全部参数
	ScopeHeaderMap = "HEADER_MAP"
	// ScopeAttr 获取Http Attributes的单个参数
	ScopeAttr = "ATTR"
	// ScopeAttrs 获取Http Attributes的Map结果
	ScopeAttrs = "ATTRS"
	// ScopeBody 获取Body数据
	ScopeBody = "BODY"
	// ScopeRequest 获取Request元数据
	ScopeRequest = "REQUEST"
	// ScopeAuto 自动查找数据源
	ScopeAuto = "AUTO"
)

// Support protocols
const (
	ProtoDubbo = "DUBBO"
	ProtoGRPC  = "GRPC"
	ProtoHttp  = "HTTP"
	ProtoEcho  = "ECHO"
	ProtoInApp = "INAPP"
)

const (
	// SpecKindService An annotation key for the meta of ServiceSpec
	SpecKindService = "flux.go/spec.meta.service"

	// SpecKindService An annotation key for the meta of EndpointSpec
	SpecKindEndpoint = "flux.go/spec.meta.endpoint"

	// SpecKindService An annotation key for documents: A brief description of the parameter.
	SpecDocDescription = "flux.go/doc.description"
)

// NamedValueSpec 定义KV键值对
type NamedValueSpec struct {
	Name  string      `json:"name" yaml:"name"`
	Value interface{} `json:"value" yaml:"value"`
}

// GetString 获取键值对的值，并转换值为String。如果值类型为列表数组，则读取第1个值并转换。
// 如果转换失败，返回空字符串。
func (a NamedValueSpec) GetString() string {
	rv := reflect.ValueOf(a.Value)
	if rv.Kind() == reflect.Slice {
		if rv.Len() == 0 {
			return ""
		}
		return cast.ToString(rv.Index(0).Interface())
	}
	return cast.ToString(a.Value)
}

func (a NamedValueSpec) GetStrings() []string {
	return cast.ToStringSlice(a.Value)
}

func (a NamedValueSpec) GetBoolean() bool {
	return cast.ToBool(a.Value)
}

func (a NamedValueSpec) IsValid() bool {
	return a.Name != "" && a.Value != nil
}

// Annotations 注解，用于声明模型的固定有属性。
type Annotations map[string]interface{}

func (a Annotations) Exists(name string) bool {
	_, ok := a[name]
	return ok
}

func (a Annotations) Get(name string) NamedValueSpec {
	if v, ok := a[name]; ok {
		return NamedValueSpec{Name: name, Value: v}
	}
	return NamedValueSpec{}
}

func (a Annotations) GetEx(name string) (NamedValueSpec, bool) {
	if v, ok := a[name]; ok {
		return NamedValueSpec{Name: name, Value: v}, true
	}
	return NamedValueSpec{}, false
}

// EncodingType Golang内置参数类型
type EncodingType string

func (m EncodingType) Contains(s string) bool {
	return strings.Contains(string(m), s)
}

const (
	EncodingTypeGoNumber      = EncodingType("go.lang/number")
	EncodingTypeGoObject      = EncodingType("go.lang/object")
	EncodingTypeGoString      = EncodingType("go.lang/string")
	EncodingTypeGoListString  = EncodingType("go.lang/[]string")
	EncodingTypeGoListObject  = EncodingType("go.lang/[]object")
	EncodingTypeGoMapString   = EncodingType("go.lang/map[string]object")
	EncodingTypeMapStringList = EncodingType("go.lang/map[string][]string")
)

// EncodeValue 包含指示值的类型和Value结构
type EncodeValue struct {
	valid    bool         // 是否有效
	Encoding EncodingType // 数据类型
	Value    interface{}  // 原始值类型
}

func NewEncodeValue(value interface{}, encoding EncodingType) EncodeValue {
	return EncodeValue{
		Value: value, Encoding: encoding, valid: !IsNil(value),
	}
}

func NewEncodeValueWith(value interface{}, encoding EncodingType, valid func() bool) EncodeValue {
	return EncodeValue{
		Value: value, Encoding: encoding, valid: valid(),
	}
}

func (v EncodeValue) IsValid() bool {
	return v.valid
}
