package internal

import (
	"bytes"
	"fmt"
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/spf13/cast"
	"net/url"
	"strings"
)

var (
	// 转义 "" 符号
	_jsonQuoteEncoder = strings.NewReplacer(`"`, `\"`)
)

// JSONFromQuery 将HttpUrlQuery字符串转换成JSON格式数据。
// Tested
func JSONFromQuery(queryStr []byte) ([]byte, error) {
	queryValues, err := url.ParseQuery(string(queryStr))
	if nil != err {
		return nil, err
	}
	fields := make([]string, 0, len(queryValues))
	for key, values := range queryValues {
		if len(values) > 1 {
			// quote with ""
			copied := make([]string, len(values))
			for i, val := range values {
				copied[i] = "\"" + string(JSONEncodeQuote(&val)) + "\""
			}
			fields = append(fields, "\""+key+"\":["+strings.Join(copied, ",")+"]")
		} else {
			fields = append(fields, "\""+key+"\":\""+string(JSONEncodeQuote(&values[0]))+"\"")
		}
	}
	bf := new(bytes.Buffer)
	bf.WriteByte('{')
	bf.WriteString(strings.Join(fields, ","))
	bf.WriteByte('}')
	return bf.Bytes(), nil
}

func JSONEncodeQuote(str *string) []byte {
	return []byte(_jsonQuoteEncoder.Replace(*str))
}

func JSONToStrMapE(data []byte) (map[string]interface{}, error) {
	var strmap = map[string]interface{}{}
	if err := ext.JSONUnmarshal(data, &strmap); nil != err {
		return nil, fmt.Errorf("cannot decode text to hashmap, text: %s, error:%w", string(data), err)
	} else {
		return strmap, nil
	}
}

func JSONObjectToStrMapE(valueobj flux.EncodeValue) (map[string]interface{}, error) {
	var m = map[string]interface{}{}
	var i = valueobj.Value
	switch v := i.(type) {
	case map[interface{}]interface{}:
		for k, val := range v {
			m[cast.ToString(k)] = val
		}
		return m, nil
	case map[string]interface{}:
		return v, nil
	case string:
		return JSONToStrMapE([]byte(v))
	default:
		return nil, fmt.Errorf("unsupported mime-type to hashmap, value: %+v, value.type:%T, mime-type: %s",
			valueobj.Value, valueobj.Value, valueobj.Encoding)
	}
}
