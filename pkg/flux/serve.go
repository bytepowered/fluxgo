package flux

import (
	"fmt"
	"net/http"
)

var _ error = new(ServeError)

// ServeError 定义网关处理请求的服务错误；
// 它包含：错误定义的状态码、错误消息、内部错误等元数据
type ServeError struct {
	StatusCode int         // 响应状态码
	ErrorCode  interface{} // 业务错误码
	Message    string      // 错误消息
	CauseError error       // 内部错误对象；错误对象不会被输出到请求端；
	Header     http.Header // 响应Header
}

func (e *ServeError) Error() string {
	if nil != e.CauseError {
		return fmt.Sprintf("ServeError: StatusCode=%d, ErrorCode=%s, Message=%s, Error=%s", e.StatusCode, e.ErrorCode, e.Message, e.CauseError)
	} else {
		return fmt.Sprintf("ServeError: StatusCode=%d, ErrorCode=%s, Message=%s", e.StatusCode, e.ErrorCode, e.Message)
	}
}

func (e *ServeError) MergeHeader(header http.Header) *ServeError {
	if e.Header == nil {
		e.Header = header.Clone()
	} else {
		for key, values := range header {
			for _, value := range values {
				e.Header.Add(key, value)
			}
		}
	}
	return e
}

type (
	// ServeResponseWriter 用于解析和序列化响应数据结构的接口，并将序列化后的数据写入Http响应流。
	ServeResponseWriter interface {
		// Write 当内部处理链完成并返回数据时，将执行此函数，通过 Context 写入业务响应数据到Http响应流。
		// Write 是请求最终函数，在 Write 函数的具体实现中：
		// 1. 应当包含对 ServeResponse 的序列化操作；
		// 2. 应当自包含向Http响应流处理过程中发生异常的处理；
		Write(ctx Context, response *ServeResponse)

		// WriteError 当内部处理链在任何阶段发生错误并返回时，将执行此函数，通过 Context 写入错误响应数据到Http响应流。
		// WriteError 是请求最终函数，在 WriteError 函数的具体实现中：
		// 1. 应当包含对 ServeError 的序列化操作；
		// 2. 应当自包含向Http响应流处理过程中发生异常的处理；
		WriteError(ctx Context, err *ServeError)
	}
	// ServeResponse 表示后端服务(Dubbo/Http/gRPC/Echo)返回响应数据结构，
	// 包含后端期望透传的状态码、Header和Attachment等数据
	ServeResponse struct {
		StatusCode  int                    // Http状态码
		Headers     http.Header            // Http Header
		Attachments map[string]interface{} // Attachment
		Body        interface{}            // 响应数据体
	}
)

func NewServeResponse(status int, body interface{}) *ServeResponse {
	return &ServeResponse{
		StatusCode:  status,
		Headers:     make(http.Header, 0),
		Attachments: make(map[string]interface{}, 0),
		Body:        body,
	}
}
