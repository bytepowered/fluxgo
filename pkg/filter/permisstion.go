package filter

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/logger"
)

const (
	TypeIdPermissionV2Filter = "permission_filter"
)

type (
	// PermissionReport 权限验证结果报告
	PermissionReport struct {
		StatusCode int    `json:"statusCode"`
		Success    bool   `json:"success"`
		ErrorCode  string `json:"errorCode"`
		Message    string `json:"message"`
	}
	// PermissionVerifyFunc 权限验证
	// @return pass 对当前请求的权限验证是否通过；
	// @return err 如果验证过程发生错误，返回error；
	PermissionVerifyFunc func(services []flux.ServiceSpec, ctx flux.Context) (report PermissionReport, err error)
)

// PermissionConfig 权限配置
type PermissionConfig struct {
	SkipFunc   flux.FilterSkipper
	VerifyFunc PermissionVerifyFunc
}

func NewPermissionVerifyReport(success bool, errorCode, message string) PermissionReport {
	return PermissionReport{
		StatusCode: flux.StatusUnauthorized,
		Success:    success,
		ErrorCode:  errorCode,
		Message:    message,
	}
}

func NewPermissionFilter(c PermissionConfig) *PermissionFilter {
	return &PermissionFilter{
		Configs: c,
	}
}

// PermissionFilter 提供基于Endpoint.Permission元数据的权限验证
type PermissionFilter struct {
	Disabled bool
	Configs  PermissionConfig
}

func (p *PermissionFilter) OnInit(config *flux.Configuration) error {
	config.SetDefaults(map[string]interface{}{
		ConfigKeyDisabled: false,
	})
	p.Disabled = config.GetBool(ConfigKeyDisabled)
	if p.Disabled {
		logger.Info("Endpoint PermissionFilter was DISABLED!!")
		return nil
	}
	if flux.IsNil(p.Configs.SkipFunc) {
		p.Configs.SkipFunc = func(_ flux.Context) bool {
			return false
		}
	}
	if flux.IsNil(p.Configs.VerifyFunc) {
		return fmt.Errorf("PermissionFilter.PermissionVerifyFunc is nil")
	}
	return nil
}

func (*PermissionFilter) FilterId() string {
	return TypeIdPermissionV2Filter
}

func (p *PermissionFilter) DoFilter(next flux.FilterInvoker) flux.FilterInvoker {
	return func(ctx flux.Context) *flux.ServeError {
		if p.Disabled || p.Configs.SkipFunc(ctx) {
			return next(ctx)
		}
		defer func() {
			ctx.AddMetric(p.FilterId(), time.Since(ctx.StartAt()))
		}()
		ids := ctx.Endpoint().Annotation(flux.EndpointAnnotationPermissions).GetStrings()
		workers := make([]flux.ServiceSpec, 0, len(ids))
		for _, id := range ids {
			if srv, ok := ext.ServiceByID(id); ok {
				workers = append(workers, srv)
			} else {
				return &flux.ServeError{
					StatusCode: flux.StatusServerError,
					ErrorCode:  flux.ErrorCodeGatewayInternal,
					Message:    flux.ErrorMessagePermissionServiceNotFound,
					CauseError: errors.New("permission.service not found, id: " + id),
				}
			}
		}
		report, err := p.Configs.VerifyFunc(workers, ctx)
		ctx.AddMetric(p.FilterId(), time.Since(ctx.StartAt()))
		if nil != err {
			if serr, ok := err.(*flux.ServeError); ok {
				return serr
			}
			return &flux.ServeError{
				StatusCode: http.StatusForbidden,
				ErrorCode:  flux.ErrorCodeGatewayInternal,
				Message:    flux.ErrorMessagePermissionVerifyError,
				CauseError: err,
			}
		}
		if !report.Success {
			return &flux.ServeError{
				StatusCode: EnsurePermissionStatusCode(report.StatusCode),
				ErrorCode:  EnsurePermissionErrorCode(report.ErrorCode),
				Message:    EnsurePermissionMessage(report.Message),
			}
		}
		return next(ctx)
	}
}

// InvokeCodec 执行权限验证的后端服务，获取响应结果；
func (p *PermissionFilter) InvokeCodec(ctx flux.Context, service flux.ServiceSpec) (*flux.ServeResponse, *flux.ServeError) {
	proto := service.Protocol
	transporter, ok := ext.TransporterByProto(proto)
	if !ok {
		return nil, &flux.ServeError{
			StatusCode: flux.StatusServerError,
			ErrorCode:  flux.ErrorCodeGatewayInternal,
			Message:    flux.ErrorMessageProtocolUnknown,
			CauseError: fmt.Errorf("unknown rpc protocol:%s", proto),
		}
	}
	return transporter.DoInvoke(ctx, service)
}

func EnsurePermissionStatusCode(status int) int {
	if status < 100 {
		return http.StatusForbidden
	}
	return status
}

func EnsurePermissionErrorCode(code string) string {
	if "" == code {
		return flux.ErrorCodePermissionDenied
	}
	return code
}

func EnsurePermissionMessage(message string) string {
	if "" == message {
		return flux.ErrorMessagePermissionAccessDenied
	}
	return message
}
