package http

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/logger"
	"github.com/bytepowered/fluxgo/pkg/toolkit"
)

func init() {
	ext.RegisterTransporter(flux.ProtoHttp,
		NewTransporterWith(
			WithTransportCodec(NewTransportCodecFunc()),
			WithAssembleRequest(DefaultAssembleRequest),
			WithAssembleHeader(DefaultAssembleHeaders),
		),
	)
}

var _ flux.Transporter = new(RpcTransporter)
var _ flux.Initializer = new(RpcTransporter)

type (
	// Option 配置函数
	Option func(service *RpcTransporter)
	// AssembleRequestFunc Http调用参数封装函数，可外部化配置为其它协议的值对象
	AssembleRequestFunc func(ctx flux.Context, service *flux.ServiceSpec) (*http.Request, error)
	// AssemblyHeadersFunc 封装Attachment为Headers的函数
	AssemblyHeadersFunc func(context flux.Context) (http.Header, error)
)

type RpcTransporter struct {
	client          *http.Client
	codec           flux.TransportCodecFunc
	trace           bool
	assembleRequest AssembleRequestFunc
	assembleHeader  AssemblyHeadersFunc
}

func NewTransporter() *RpcTransporter {
	return &RpcTransporter{
		client: &http.Client{
			Timeout: time.Second * 10,
		},
		codec:           NewTransportCodecFunc(),
		assembleRequest: DefaultAssembleRequest,
		assembleHeader:  DefaultAssembleHeaders,
	}
}

func NewTransporterWith(opts ...Option) *RpcTransporter {
	bts := NewTransporter()
	for _, opt := range opts {
		opt(bts)
	}
	return bts
}

// WithHttpClient 用于配置HttpClient客户端
func WithHttpClient(client *http.Client) Option {
	return func(s *RpcTransporter) {
		s.client = client
	}
}

// WithTransportCodec 用于配置响应数据解析实现函数
func WithTransportCodec(fun flux.TransportCodecFunc) Option {
	return func(service *RpcTransporter) {
		service.codec = fun
	}
}

// WithAssembleRequest 用于配置转发Http请求参数封装实现函数
func WithAssembleRequest(fun AssembleRequestFunc) Option {
	return func(service *RpcTransporter) {
		service.assembleRequest = fun
	}
}

// WithAssembleHeader 用于配置转发Http请求参数封装实现函数
func WithAssembleHeader(fun AssemblyHeadersFunc) Option {
	return func(service *RpcTransporter) {
		service.assembleHeader = fun
	}
}

func (b *RpcTransporter) OnInit(config *flux.Configuration) error {
	b.trace = config.GetBool("trace_enable")
	flux.AssertNotNil(b.codec, "<TransportCodecFunc> MUST NOT nil")
	flux.AssertNotNil(b.assembleHeader, "<AssemblyHeadersFunc> MUST NOT nil")
	flux.AssertNotNil(b.assembleRequest, "<AssembleRequestFunc> MUST NOT nil")
	return nil
}

func (b *RpcTransporter) DoInvoke(ctx flux.Context, service flux.ServiceSpec) (*flux.ServeResponse, *flux.ServeError) {
	invret, inverr := b.invoke0(ctx, service)
	if inverr != nil {
		return nil, inverr
	}
	// decode response
	decret, decerr := b.codec(ctx, invret, make(map[string]interface{}, 0))
	if nil != decerr {
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayInternal,
			Message:    flux.ErrorMessageTransportCodecError,
			CauseError: fmt.Errorf("decode http response, err: %w", decerr),
		}
	}
	return decret, nil
}

func (b *RpcTransporter) invoke0(ctx flux.Context, service flux.ServiceSpec) (interface{}, *flux.ServeError) {
	flux.AssertNotEmpty(service.Url, "<service.url> MUST NOT empty in http transporter")
	// request
	newRequest, err := b.assembleRequest(ctx, &service)
	if nil != err {
		logger.TraceExtras(ctx.RequestId(), map[string]string{
			"invoke.service":     service.ServiceID(),
			"invoke.service.url": service.Url,
		}).Errorw("TRANSPORTER:HTTP:ASSEMBLE/header", "error", err)
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayInternal,
			Message:    flux.ErrorMessageTransportHttpAssembleFailed,
			CauseError: err,
		}
	}
	trace := logger.TraceExtras(ctx.RequestId(), map[string]string{
		"invoke.service": service.ServiceID(),
		"invoke.http":    newRequest.Method + ":" + newRequest.URL.String(),
	})
	// header
	header, err := b.assembleHeader(ctx)
	if err != nil {
		trace.Errorw("TRANSPORTER:HTTP:ASSEMBLE/header", "error", err)
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayInternal,
			Message:    flux.ErrorMessageTransportHttpAssembleFailed,
			CauseError: err,
		}
	}
	for k, s := range header {
		for _, v := range s {
			newRequest.Header.Add(k, v)
		}
	}
	if b.trace {
		bodys := string(toolkit.ReadReaderBytes(ctx.BodyReader()))
		trace.Infow("TRANSPORTER:HTTP:INVOKE/args",
			"arg-query", newRequest.URL.RawQuery, "arg-body", bodys, "arg-header", header)
	}
	return b.execute(newRequest)
}

func (b *RpcTransporter) execute(request *http.Request) (interface{}, *flux.ServeError) {
	resp, err := b.client.Do(request)
	if nil != err {
		msg := flux.ErrorMessageTransportHttpInvokeFailed
		if uErr, ok := err.(*url.Error); ok {
			msg = fmt.Sprintf("HTTPEX:REMOTE_ERROR:%s", uErr.Error())
		}
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayTransporter,
			Message:    msg,
			CauseError: err,
		}
	}
	return resp, nil
}
