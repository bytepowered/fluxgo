package filter

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

import (
	"github.com/bytepowered/fluxgo/pkg/common"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"github.com/spf13/cast"
)

const (
	TypeIdJWTFilter = "jwt_filter"
)

const (
	FeatureJWT             = "feature:jwt"
	ConfigKeyAttachmentKey = "attachment_key"
)

var _ flux.Filter = new(JWTFilter)

type JWTConfig struct {
	AttKeyPrefix string
	// 默认查找Token的函数
	TokenExtractor func(ctx flux.Context) (string, error)
	// 加载签名验证密钥的函数
	SecretKeyLoader func(ctx flux.Context, token *jwt.Token) (interface{}, error)
}

func NewJWTFilter(config JWTConfig) *JWTFilter {
	return &JWTFilter{
		Config: config,
	}
}

type JWTFilter struct {
	Config JWTConfig
}

func (f *JWTFilter) FilterId() string {
	return TypeIdJWTFilter
}

func (f *JWTFilter) OnInit(config *flux.Configuration) error {
	if f.Config.TokenExtractor == nil {
		f.Config.TokenExtractor = func(ctx flux.Context) (string, error) {
			return ExtractTokenOAuth2(ctx)
		}
	}
	// ClaimsKeyPrefix
	if "" == f.Config.AttKeyPrefix {
		f.Config.AttKeyPrefix = cast.ToString(config.GetOrDefault(ConfigKeyAttachmentKey, "jwt"))
	}
	flux.AssertNotNil(f.Config.SecretKeyLoader, "<secret-loader> must not nil")
	return nil
}

func (f *JWTFilter) DoFilter(next flux.FilterInvoker) flux.FilterInvoker {
	return func(ctx flux.Context) *flux.ServeError {
		defer func() {
			ctx.AddMetric(f.FilterId(), time.Since(ctx.StartAt()))
		}()
		tokenStr, err := f.Config.TokenExtractor(ctx)
		// 启用JWT特性，但没有传Token参数
		if tokenStr == "" || err == request.ErrNoTokenInRequest {
			return &flux.ServeError{
				StatusCode: http.StatusUnauthorized,
				ErrorCode:  flux.ErrorCodeJwtNotFound,
				Message:    "JWT:VALIDATE: token not found",
			}
		}
		ctx.Logger().Infow("AUTHORIZATION:JWT:TOKEN_VERIFY", "token", tokenStr)
		// 解析和校验
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return f.Config.SecretKeyLoader(ctx, token)
		})
		if token != nil && token.Valid {
			// set claims to attributes
			ctx.Logger().Infow("AUTHORIZATION:JWT:VALIDATE:PASSED", "jwt.claims", claims)
			for k, v := range claims {
				ctx.SetAttribute(f.Config.AttKeyPrefix+"."+k, v)
			}
			return next(ctx)
		} else {
			ctx.Logger().Infow("AUTHORIZATION:JWT:VALIDATE:REJECTED", "error", err)
		}
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return &flux.ServeError{
					StatusCode: http.StatusBadRequest,
					ErrorCode:  flux.ErrorCodeJwtMalformed,
					Message:    "JWT:VALIDATE: token malformed",
				}
			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				// Token is either expired or not active yet
				return &flux.ServeError{
					StatusCode: http.StatusUnauthorized,
					ErrorCode:  flux.ErrorCodeJwtExpired,
					Message:    "JWT:VALIDATE:token is expired/not active",
				}
			} else {
				return &flux.ServeError{
					StatusCode: http.StatusBadRequest,
					ErrorCode:  flux.ErrorCodeJwtMalformed,
					Message:    "JWT:VALIDATE: Couldn't handle(002)",
					CauseError: err,
				}
			}
		} else {
			return &flux.ServeError{
				StatusCode: http.StatusBadRequest,
				ErrorCode:  flux.ErrorCodeJwtMalformed,
				Message:    "JWT:VALIDATE: Couldn't handle(001)",
				CauseError: err,
			}
		}
	}
}

// ExtractTokenOAuth2 按OAuth2请求，从Header:Authorization和form:access_token中抓取Token
func ExtractTokenOAuth2(ctx flux.Context) (string, error) {
	return request.OAuth2Extractor.ExtractToken(ctx.Request())
}

// ExtractTokenByFeature 根据Endpoint属性配置抓取Token的值
func ExtractTokenByFeature(ctx flux.Context) (string, error) {
	expr := ctx.Endpoint().Annotation(FeatureJWT).GetString()
	if "" == expr {
		return "", fmt.Errorf("<%s> not found in endpoint.attrs", FeatureJWT)
	}
	return TokenStripBearerPrefix(common.LookupWebValueByExpr(ctx, expr))
}

// TokenStripBearerPrefix Strips 'Bearer ' prefix from bearer token string
func TokenStripBearerPrefix(token string) (string, error) {
	// Should be a bearer token
	if len(token) > 6 && strings.ToUpper(token[0:7]) == "BEARER " {
		return token[7:], nil
	}
	return token, nil
}
