package dubbo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"reflect"
	"regexp"
	"sync"
	"time"
)

import (
	"github.com/apache/dubbo-go/common"
	"github.com/apache/dubbo-go/common/constant"
	"github.com/apache/dubbo-go/common/extension"
	"github.com/apache/dubbo-go/common/proxy"
	"github.com/apache/dubbo-go/common/proxy/proxy_factory"
	dubgo "github.com/apache/dubbo-go/config"
	"github.com/apache/dubbo-go/protocol/dubbo"
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/logger"
	jsoniter "github.com/json-iterator/go"
)

import (
	_ "github.com/apache/dubbo-go/cluster/cluster_impl"
	_ "github.com/apache/dubbo-go/cluster/loadbalance"
	_ "github.com/apache/dubbo-go/filter/filter_impl"
	_ "github.com/apache/dubbo-go/registry/protocol"
)

const (
	ConfigKeyTraceEnable    = "trace_enable"
	ConfigKeyReferenceDelay = "reference_delay"
)

var (
	ErrDecodeInvalidHeaders = errors.New(flux.ErrorMessageTransportDubboDecodeInvalidHeader)
	ErrDecodeInvalidStatus  = errors.New(flux.ErrorMessageTransportDubboDecodeInvalidStatus)
)

var (
	_     flux.Transporter = new(RpcTransporter)
	_json                  = jsoniter.ConfigCompatibleWithStandardLibrary
)

func init() {
	ext.RegisterTransporter(flux.ProtoDubbo, NewTransporter())
	// 替换Dubbo泛调用默认实现
	extension.SetProxyFactory("default", func(_ ...proxy.Option) proxy.ProxyFactory {
		return new(proxy_factory.GenericProxyFactory)
	})
}

type (
	// Option func to set option
	Option func(*RpcTransporter)
	// AssembleArgumentsFunc Dubbo调用参数封装函数，可外部化配置为其它协议的值对象
	AssembleArgumentsFunc func(context flux.Context, arguments []flux.ServiceArgumentSpec) (types []string, values interface{}, err error)
	// AssembleAttachmentsFunc 封装Attachment附件的函数
	AssembleAttachmentsFunc func(context flux.Context, service *flux.ServiceSpec) (map[string]interface{}, error)
)

type (
	// GenericOptionsFunc DubboReference配置函数，可外部化配置Dubbo Reference
	GenericOptionsFunc func(*flux.ServiceSpec, *flux.Configuration, *dubgo.ReferenceConfig) *dubgo.ReferenceConfig
	// GenericServiceFunc 用于构建DubboGo泛型调用Service实例
	GenericServiceFunc func(*flux.ServiceSpec) common.RPCService
	// GenericInvokeFunc 用于执行Dubbo泛调用方法，返回统一Result数据结构
	GenericInvokeFunc func(ctx context.Context, args []interface{}, rpc common.RPCService) (interface{}, map[string]interface{}, error)
)

// RpcTransporter 集成DubboRPC框架的TransporterService
type RpcTransporter struct {
	// 可外部配置
	defaults         map[string]interface{}  // 配置默认值
	registry         map[string]string       // Registry的别名
	optionsFunc      []GenericOptionsFunc    // Dubbo Reference 配置函数
	serviceFunc      GenericServiceFunc      // Dubbo Service 构建函数
	invokeFunc       GenericInvokeFunc       // 执行Dubbo泛调用的函数
	argsAssembleFunc AssembleArgumentsFunc   // Dubbo参数封装函数
	attrAssembleFunc AssembleAttachmentsFunc // Attachment封装函数
	codec            flux.TransportCodecFunc // 解析响应结果的函数
	// 内部私有
	trace         bool
	configuration *flux.Configuration
	servmx        sync.RWMutex
}

// WithAssembleArgumentsFunc 用于配置Dubbo参数封装实现函数
func WithAssembleArgumentsFunc(fun AssembleArgumentsFunc) Option {
	return func(service *RpcTransporter) {
		service.argsAssembleFunc = fun
	}
}

// WithAssembleAttachmentsFunc 用于配置Attachment封装实现函数
func WithAssembleAttachmentsFunc(fun AssembleAttachmentsFunc) Option {
	return func(service *RpcTransporter) {
		service.attrAssembleFunc = fun
	}
}

// WithTransportCodecFunc 用于配置响应数据解析实现函数
func WithTransportCodecFunc(fun flux.TransportCodecFunc) Option {
	return func(service *RpcTransporter) {
		service.codec = fun
	}
}

// WithGenericOptionsFunc 用于配置DubboReference的参数配置函数
func WithGenericOptionsFunc(fun GenericOptionsFunc) Option {
	return func(service *RpcTransporter) {
		service.optionsFunc = append(service.optionsFunc, fun)
	}
}

// WithGenericServiceFunc 用于构建DubboRPCService的函数
func WithGenericServiceFunc(fun GenericServiceFunc) Option {
	return func(service *RpcTransporter) {
		service.serviceFunc = fun
	}
}

// WithGenericInvokeFunc 用于调用DubboRPCService执行方法的函数
func WithGenericInvokeFunc(fun GenericInvokeFunc) Option {
	return func(service *RpcTransporter) {
		service.invokeFunc = fun
	}
}

// WithRegistryAlias 用于配置DubboRegistry注册中心的配置别名
func WithRegistryAlias(alias map[string]string) Option {
	return func(service *RpcTransporter) {
		service.registry = alias
	}
}

// WithDefaults 用于配置默认配置值
func WithDefaults(defaults map[string]interface{}) Option {
	return func(service *RpcTransporter) {
		service.defaults = defaults
	}
}

// NewTransporterWith New dubbo transporter service with options
func NewTransporterWith(opts ...Option) flux.Transporter {
	bts := &RpcTransporter{
		optionsFunc: make([]GenericOptionsFunc, 0),
	}
	for _, opt := range opts {
		opt(bts)
	}
	return bts
}

// NewTransporter New dubbo transporter instance
func NewTransporter() flux.Transporter {
	return NewTransporterOverride()
}

// NewTransporterOverride New dubbo transporter instance
func NewTransporterOverride(overrides ...Option) flux.Transporter {
	opts := []Option{
		WithRegistryAlias(map[string]string{
			"id":       "dubbo.registry.id",
			"protocol": "dubbo.registry.protocol",
			"group":    "dubbo.registry.group",
			"timeout":  "dubbo.registry.timeout",
			"address":  "dubbo.registry.address",
			"username": "dubbo.registry.username",
			"password": "dubbo.registry.password",
		}),
		WithDefaults(map[string]interface{}{
			ConfigKeyReferenceDelay: time.Millisecond * 10,
			ConfigKeyTraceEnable:    false,
			"timeout":               "5000",
			"retries":               "0",
			"cluster":               "failover",
			"load_balance":          "random",
			"protocol":              dubbo.DUBBO,
		}),
		// 使用带Result结果的RPCService实现
		WithGenericServiceFunc(func(service *flux.ServiceSpec) common.RPCService {
			return dubgo.NewGenericService2(service.Interface)
		}),
		// 转换为 GenericService2 的调用
		WithGenericInvokeFunc(func(ctx context.Context, args []interface{}, service common.RPCService) (interface{}, map[string]interface{}, error) {
			return service.(*dubgo.GenericService2).Invoke(ctx, args)
		}),
		WithAssembleArgumentsFunc(DefaultArgumentsAssembleFunc),
		WithAssembleAttachmentsFunc(DefaultAssembleAttachmentFunc),
		WithTransportCodecFunc(NewTransportCodecFunc()),
	}
	return NewTransporterWith(append(opts, overrides...)...)
}

// OnInit init transporter
func (b *RpcTransporter) OnInit(config *flux.Configuration) error {
	logger.Info("TRANSPORTER:DUBBO:INIT")
	config.SetDefaults(b.defaults)
	b.configuration = config
	b.trace = config.GetBool(ConfigKeyTraceEnable)
	logger.Infow("TRANSPORTER:DUBBO:INIT/trace", "enable", b.trace)
	// Set default impl if not present
	if nil == b.optionsFunc {
		b.optionsFunc = make([]GenericOptionsFunc, 0)
	}
	if flux.IsNil(b.argsAssembleFunc) {
		b.argsAssembleFunc = DefaultArgumentsAssembleFunc
	}
	// 修改默认Consumer配置
	consumerc := dubgo.GetConsumerConfig()
	// 支持定义Registry
	registry := b.configuration.Sub("registry")
	registry.SetKeyAlias(b.registry)
	if id, rconfig := newConsumerRegistry(registry); id != "" && nil != rconfig {
		consumerc.Registries[id] = rconfig
		logger.Infow("TRANSPORTER:DUBBO:INIT/registry", "id", id, "config", rconfig)
	}
	dubgo.SetConsumerConfig(consumerc)
	return nil
}

// OnStartup startup service
func (b *RpcTransporter) OnStartup() error {
	flux.AssertNotNil(b.codec, "<codec-func> must not nil")
	flux.AssertNotNil(b.argsAssembleFunc, "<arguments-assemble-func> must not nil")
	flux.AssertNotNil(b.attrAssembleFunc, "<attachment-assemble-func> must not nil")
	return nil
}

// OnShutdown shutdown service
func (b *RpcTransporter) OnShutdown(_ context.Context) error {
	dubgo.BeforeShutdown()
	return nil
}

func (b *RpcTransporter) DoInvoke(ctx flux.Context, service flux.ServiceSpec) (*flux.ServeResponse, *flux.ServeError) {
	flux.AssertNotEmpty(service.Protocol, "<service.proto> is required")
	trace := logger.TraceExtras(ctx.RequestId(), map[string]string{
		"invoke.service": service.ServiceID(),
	})
	types, values, err := b.argsAssembleFunc(ctx, service.Arguments)
	if nil != err {
		trace.Errorw("TRANSPORTER:DUBBO:ASSEMBLE/arguments", "error", err)
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayInternal,
			Message:    flux.ErrorMessageTransportDubboAssembleFailed,
			CauseError: err,
		}
	}
	attachments, err := b.attrAssembleFunc(ctx, &service)
	if nil != err {
		trace.Errorw("TRANSPORTER:DUBBO:ASSEMBLE/attachments", "error", err)
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayInternal,
			Message:    flux.ErrorMessageTransportDubboAssembleFailed,
			CauseError: err,
		}
	}
	// Invoke
	if b.trace {
		invargs := map[string]interface{}{
			"names": strings.Join(func() []string {
				names := make([]string, len(service.Arguments))
				for i := 0; i < len(service.Arguments); i++ {
					names[i] = service.Arguments[i].Name
				}
				return names
			}(), ","),
			"types":  types,
			"values": values,
		}
		trace.Infow("TRANSPORTER:DUBBO:INVOKE/args", "invarg", invargs, "invattrs", attachments)
	}
	invret, invatt, inverr := b.invoke0(ctx, service, types, values, attachments)
	if b.trace && inverr == nil && invret != nil {
		data := invret
		if text, err := _json.MarshalToString(data); nil == err {
			trace.Infow("TRANSPORTER:DUBBO:INVOKE/recv", "response.json", text)
		} else {
			trace.Infow("TRANSPORTER:DUBBO:INVOKE/recv", "response.type", reflect.TypeOf(data), "response.data", fmt.Sprintf("%+v", data))
		}
	}
	select {
	case <-ctx.Context().Done():
		trace.Info("TRANSPORTER:DUBBO:INVOKE/canceled")
		return nil, &flux.ServeError{
			StatusCode: flux.StatusBadRequest,
			ErrorCode:  flux.ErrorCodeRequestCanceled,
			Message:    flux.ErrorMessageTransportDubboClientCanceled,
			CauseError: ctx.Context().Err(),
		}
	default:
		break
	}
	if nil != inverr {
		trace.Errorw("TRANSPORTER:DUBBO:INVOKE/error", "error", inverr.CauseError)
		return nil, inverr
	}
	// Codec
	// check if disable codec
	if disable, ok := ctx.Variable("disable.codec").(bool); ok && disable {
		return &flux.ServeResponse{
			StatusCode: flux.StatusOK, Attachments: invatt, Body: invret,
		}, nil
	}
	codecd, coderr := b.codec(ctx, invret, invatt)
	if nil != coderr {
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayInternal,
			Message:    flux.ErrorMessageTransportCodecError,
			CauseError: fmt.Errorf("decode dubbo response, err: %w", coderr),
		}
	}
	flux.AssertNotNil(codecd, "dubbo: <result> must not nil, request-id: "+ctx.RequestId())
	return codecd, nil
}

func (b *RpcTransporter) invoke0(ctx flux.Context, service flux.ServiceSpec, types []string, values, attachments interface{}) (interface{}, map[string]interface{}, *flux.ServeError) {
	generic := b.LoadGenericService(&service)
	goctx := context.WithValue(ctx.Context(), constant.AttachmentKey, attachments)
	ret, att, cause := b.invokeFunc(goctx, []interface{}{service.Method, types, values}, generic)
	if cause != nil {
		return nil, nil, &flux.ServeError{
			StatusCode: flux.StatusBadGateway,
			ErrorCode:  flux.ErrorCodeGatewayTransporter,
			Message:    flux.ErrorMessageTransportDubboInvokeFailed,
			CauseError: cause,
		}
	}
	return ret, att, nil
}

// LoadGenericService create and cache dubbo generic service
func (b *RpcTransporter) LoadGenericService(service *flux.ServiceSpec) common.RPCService {
	b.servmx.Lock()
	defer b.servmx.Unlock()
	if srv := dubgo.GetConsumerService(service.Interface); nil != srv {
		return srv
	}
	newRef := NewReference(service.Interface, service, b.configuration)
	// Options
	const msg = "Dubbo option-func return nil reference"
	for _, optsFunc := range b.optionsFunc {
		if nil != optsFunc {
			newRef = flux.MustNotNil(optsFunc(service, b.configuration, newRef), msg).(*dubgo.ReferenceConfig)
		}
	}
	logger.Infow("DUBBO:GENERIC:CREATE: PREPARE", "interface", service.Interface)
	srv := b.serviceFunc(service)
	dubgo.SetConsumerService(srv)
	newRef.Refer(srv)
	newRef.Implement(srv)
	t := b.configuration.GetDuration(ConfigKeyReferenceDelay)
	if t == 0 {
		t = time.Millisecond * 10
	}
	<-time.After(t)
	logger.Infow("DUBBO:GENERIC:CREATE: OK", "interface", service.Interface)
	return srv
}

func newConsumerRegistry(config *flux.Configuration) (string, *dubgo.RegistryConfig) {
	if !config.IsSet("id", "protocol") {
		return "", nil
	}
	return config.GetString("id"), &dubgo.RegistryConfig{
		Protocol:   config.GetString("protocol"),
		TimeoutStr: config.GetString("timeout"),
		Group:      config.GetString("group"),
		TTL:        config.GetString("ttl"),
		Address:    config.GetString("address"),
		Username:   config.GetString("username"),
		Password:   config.GetString("password"),
		Simplified: config.GetBool("simplified"),
		Preferred:  config.GetBool("preferred"),
		Zone:       config.GetString("zone"),
		Weight:     config.GetInt64("weight"),
		Params:     config.GetStringMapString("params"),
	}
}

func NewReference(refid string, service *flux.ServiceSpec, config *flux.Configuration) *dubgo.ReferenceConfig {
	logger.Infow("DUBBO:GENERIC:CREATE:NEWREF",
		"rpc-service", service.Interface, "rpc-url", service.Url, "rpc-proto", service.Protocol,
		"rpc-group", service.Annotation(flux.ServiceAnnotationRpcGroup).GetString(),
		"rpc-version", service.Annotation(flux.ServiceAnnotationRpcVersion).GetString())
	ref := dubgo.NewReferenceConfig(refid, context.Background())
	// 订正 url 地址
	if service.Url != "" {
		if hasproto(service.Url) {
			ref.Url = service.Url
		} else {
			ref.Url = service.Protocol + "://" + service.Url
		}
	}
	ref.Protocol = service.Protocol
	ref.InterfaceName = service.Interface
	ref.Version = service.Annotation(flux.ServiceAnnotationRpcVersion).GetString()
	ref.Group = service.Annotation(flux.ServiceAnnotationRpcGroup).GetString()
	ref.RequestTimeout = service.Annotation(flux.ServiceAnnotationRpcTimeout).GetString()
	ref.Retries = service.Annotation(flux.ServiceAnnotationRpcRetries).GetString()
	ref.Cluster = config.GetString("cluster")
	ref.Protocol = config.GetString("protocol")
	ref.Loadbalance = config.GetString("load_balance")
	ref.Generic = true
	return ref
}

var pattern = regexp.MustCompile(`^[a-zA-Z1-9]{2,}://`)

func hasproto(s string) bool {
	return pattern.Match([]byte(s))
}
