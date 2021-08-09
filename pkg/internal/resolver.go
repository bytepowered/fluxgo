package internal

import (
	"errors"
	"fmt"
	"github.com/spf13/cast"
	"io"
	"io/ioutil"
	"reflect"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
)

const (
	JavaLangObjectClassName  = "java.lang.Object"
	JavaLangStringClassName  = "java.lang.String"
	JavaLangIntegerClassName = "java.lang.Integer"
	JavaLangLongClassName    = "java.lang.Long"
	JavaLangFloatClassName   = "java.lang.Float"
	JavaLangDoubleClassName  = "java.lang.Double"
	JavaLangBooleanClassName = "java.lang.Boolean"
	JavaUtilMapClassName     = "java.util.Map"
	JavaUtilListClassName    = "java.util.List"
	JavaIOSerializable       = "java.io.Serializable"
)

var (
	errCastToByteTypeNotSupported = errors.New("cannot convert value to []byte")
)

var (
	objectResolver = ext.EncodeValueResolver(func(valueobj flux.EncodeValue, _ string, genericTypes []string) (interface{}, error) {
		return valueobj.Value, nil
	})
	stringResolver = ext.EncodeValueResolver(func(valueobj flux.EncodeValue, _ string, genericTypes []string) (interface{}, error) {
		return CastDecodeEncodeValueToString(valueobj)
	})
	integerResolver = ext.WrapEncodeValueResolver(func(value interface{}) (interface{}, error) {
		if isEmptyOrNil(value) {
			return int(0), nil
		}
		return cast.ToIntE(value)
	}).ResolveTo
	longResolver = ext.WrapEncodeValueResolver(func(value interface{}) (interface{}, error) {
		if isEmptyOrNil(value) {
			return int64(0), nil
		}
		return cast.ToInt64E(value)
	}).ResolveTo
	float32Resolver = ext.WrapEncodeValueResolver(func(value interface{}) (interface{}, error) {
		if isEmptyOrNil(value) {
			return float32(0), nil
		}
		return cast.ToFloat32E(value)
	}).ResolveTo
	float64Resolver = ext.WrapEncodeValueResolver(func(value interface{}) (interface{}, error) {
		if isEmptyOrNil(value) {
			return float64(0), nil
		}
		return cast.ToFloat64E(value)
	}).ResolveTo
	booleanResolver = ext.WrapEncodeValueResolver(func(value interface{}) (interface{}, error) {
		if isEmptyOrNil(value) {
			return false, nil
		}
		return cast.ToBoolE(value)
	}).ResolveTo
	mapResolver = ext.EncodeValueResolver(func(valueobj flux.EncodeValue, _ string, genericTypes []string) (interface{}, error) {
		return ToStringMapE(valueobj)
	})
	listResolver = ext.EncodeValueResolver(func(valueobj flux.EncodeValue, _ string, genericTypes []string) (interface{}, error) {
		return ToGenericListE(genericTypes, valueobj)
	})
	complexObjectResolver = ext.EncodeValueResolver(func(valueobj flux.EncodeValue, class string, generic []string) (interface{}, error) {
		if isEmptyOrNil(valueobj.Value) {
			return map[string]interface{}{"class": class}, nil
		}
		sm, err := ToStringMapE(valueobj)
		sm["class"] = class
		if nil != err {
			return nil, err
		}
		return sm, nil
	})
)

func init() {
	ext.RegisterEncodeValueResolver("string", stringResolver)
	ext.RegisterEncodeValueResolver(JavaLangStringClassName, stringResolver)

	ext.RegisterEncodeValueResolver("int", integerResolver)
	ext.RegisterEncodeValueResolver(JavaLangIntegerClassName, integerResolver)

	ext.RegisterEncodeValueResolver("int64", longResolver)
	ext.RegisterEncodeValueResolver("long", longResolver)
	ext.RegisterEncodeValueResolver(JavaLangLongClassName, longResolver)

	ext.RegisterEncodeValueResolver("float", float32Resolver)
	ext.RegisterEncodeValueResolver("float32", float32Resolver)
	ext.RegisterEncodeValueResolver(JavaLangFloatClassName, float32Resolver)

	ext.RegisterEncodeValueResolver("float64", float64Resolver)
	ext.RegisterEncodeValueResolver("double", float64Resolver)
	ext.RegisterEncodeValueResolver(JavaLangDoubleClassName, float64Resolver)

	ext.RegisterEncodeValueResolver("bool", booleanResolver)
	ext.RegisterEncodeValueResolver("boolean", booleanResolver)
	ext.RegisterEncodeValueResolver(JavaLangBooleanClassName, booleanResolver)

	ext.RegisterEncodeValueResolver("map", mapResolver)
	ext.RegisterEncodeValueResolver(JavaUtilMapClassName, mapResolver)

	ext.RegisterEncodeValueResolver("slice", listResolver)
	ext.RegisterEncodeValueResolver("list", listResolver)
	ext.RegisterEncodeValueResolver(JavaUtilListClassName, listResolver)

	ext.RegisterEncodeValueResolver(JavaIOSerializable, objectResolver)
	ext.RegisterEncodeValueResolver(JavaLangObjectClassName, objectResolver)

	ext.RegisterEncodeValueResolver(ext.DefaultEncodeValueResolverName, complexObjectResolver)
}

// CastDecodeEncodeValueToString 最大努力地将值转换成String类型。
// 如果类型无法安全地转换成String或者解析异常，返回错误。
func CastDecodeEncodeValueToString(valueobj flux.EncodeValue) (string, error) {
	if isEmptyOrNil(valueobj.Value) {
		return "", nil
	}
	// 可直接转String类型：
	if str, err := cast.ToStringE(valueobj.Value); nil == err {
		return str, nil
	}
	if data, err := toByteArray0(valueobj.Value); nil == err {
		return string(data), nil
	} else if err != errCastToByteTypeNotSupported {
		return "", err
	}
	if data, err := ext.JSONMarshal(valueobj.Value); nil != err {
		return "", err
	} else {
		return string(data), nil
	}
}

// ToStringMapE 最大努力地将值转换成map[string]any类型。
// 如果类型无法安全地转换成map[string]any或者解析异常，返回错误。
func ToStringMapE(valueobj flux.EncodeValue) (map[string]interface{}, error) {
	if isEmptyOrNil(valueobj.Value) || !valueobj.IsValid() {
		return make(map[string]interface{}, 0), nil
	}
	switch valueobj.Encoding {
	case flux.EncodingTypeMapStringList:
		omap, ok := valueobj.Value.(map[string][]string)
		flux.AssertM(ok, func() string {
			return fmt.Sprintf("mt-value(define:%s) is not map[string][]string, mt-value:%+v", valueobj.Encoding, valueobj.Value)
		})
		var cmap = make(map[string]interface{}, len(omap))
		for k, v := range omap {
			cmap[k] = v
		}
		return cmap, nil

	case flux.EncodingTypeGoMapString:
		return cast.ToStringMap(valueobj.Value), nil

	case flux.EncodingTypeGoString:
		str, ok := valueobj.Value.(string)
		flux.AssertM(ok, func() string {
			return fmt.Sprintf("mt-value(define:%s) is not go:string, mt-value:%+v", valueobj.Encoding, valueobj.Value)
		})
		return JSONToStrMapE([]byte(str))

	case flux.EncodingTypeGoObject:
		return JSONObjectToStrMapE(valueobj)

	default:
		if valueobj.Encoding.Contains("application/json") {
			data, err := toByteArray(valueobj.Value)
			if nil != err {
				return nil, err
			}
			return JSONToStrMapE(data)
		} else if valueobj.Encoding.Contains("application/x-www-form-urlencoded") {
			data, err := toByteArray(valueobj.Value)
			if nil != err {
				return nil, err
			}
			if jsonbytes, err := JSONFromQuery(data); nil != err {
				return nil, err
			} else {
				return JSONToStrMapE(jsonbytes)
			}
		} else {
			return JSONObjectToStrMapE(valueobj)
		}
	}
}

// ToGenericListE 最大努力地将值转换成[]any类型。
// 如果类型无法安全地转换成[]any或者解析异常，返回错误。
func ToGenericListE(generics []string, valueobj flux.EncodeValue) (interface{}, error) {
	if isEmptyOrNil(valueobj.Value) {
		return make([]interface{}, 0), nil
	}
	vType := reflect.TypeOf(valueobj.Value)
	// 没有指定泛型类型
	if len(generics) == 0 {
		return []interface{}{valueobj.Value}, nil
	}
	// 进行特定泛型类型转换
	generic := generics[0]
	resolver := ext.EncodeValueResolverByType(generic)
	kind := vType.Kind()
	if kind == reflect.Slice {
		vValue := reflect.ValueOf(valueobj.Value)
		out := make([]interface{}, vValue.Len())
		for i := 0; i < vValue.Len(); i++ {
			if v, err := resolver(ext.NewObjectEncodeValue(vValue.Index(i).Interface()), generic, []string{}); nil != err {
				return nil, err
			} else {
				out[i] = v
			}
		}
		return out, nil
	}
	if v, err := resolver(valueobj, generic, []string{}); nil != err {
		return nil, err
	} else {
		return []interface{}{v}, nil
	}
}

func toByteArray(v interface{}) ([]byte, error) {
	if bs, err := toByteArray0(v); nil != err {
		return nil, fmt.Errorf("value: %+v, value.type:%T, error: %w", v, v, err)
	} else {
		return bs, nil
	}
}

func toByteArray0(v interface{}) ([]byte, error) {
	switch v.(type) {
	case []byte:
		return v.([]byte), nil
	case string:
		return []byte(v.(string)), nil
	case io.Reader:
		data, err := ioutil.ReadAll(v.(io.Reader))
		if closer, ok := v.(io.Closer); ok {
			_ = closer.Close()
		}
		if nil != err {
			return nil, err
		} else {
			return data, nil
		}
	default:
		return nil, errCastToByteTypeNotSupported
	}
}

func isEmptyOrNil(v interface{}) bool {
	if s, ok := v.(string); ok {
		return "" == s
	}
	return nil == v
}
