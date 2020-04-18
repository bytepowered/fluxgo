package filter

import (
	"github.com/bytepowered/flux"
	"github.com/bytepowered/flux/logger"
	"github.com/bytepowered/lakego"
	"golang.org/x/time/rate"
	"net/http"
	"time"
)

const (
	keyConfigLimitLookupId = "limit-lookup"
	keyConfigLimitRateKey  = "limit-rate"
	keyConfigLimitSizeKey  = "limit-size"
)

const (
	FilterIdRateLimitFilter = "RateLimitFilter"
)

func RateLimitFilterFactory() interface{} {
	return NewRateLimitFilter()
}

func NewRateLimitFilter() flux.Filter {
	return new(RateLimitFilter)
}

type RateLimitConfig struct {
	lookupId  string
	limitRate time.Duration
	limitSize int
}

type RateLimitFilter struct {
	config   *RateLimitConfig
	limiters lakego.Cache
}

func (r *RateLimitFilter) Init(config flux.Config) error {
	rateDuration, err := time.ParseDuration(config.StringOrDefault(keyConfigLimitRateKey, "1m"))
	if err != nil {
		return err
	}
	r.config = &RateLimitConfig{
		lookupId:  config.StringOrDefault(keyConfigLimitLookupId, flux.XJwtSubject),
		limitRate: rateDuration,
		limitSize: int(config.Int64OrDefault(keyConfigLimitSizeKey, 1000)),
	}
	logger.Infof("JWT filter initializing, config: %+v", r.config)
	// RateLimit缓存大小
	r.limiters = lakego.NewSimple()
	return nil
}

func (r *RateLimitFilter) Invoke(next flux.FilterInvoker) flux.FilterInvoker {
	return func(ctx flux.Context) *flux.InvokeError {
		id := LookupValue(r.config.lookupId, ctx)
		limit, _ := r.limiters.GetOrLoad(id, func(_ lakego.Key) (lakego.Value, error) {
			return rate.NewLimiter(rate.Every(r.config.limitRate), r.config.limitSize), nil
		})
		if limit.(*rate.Limiter).Allow() {
			return next(ctx)
		} else {
			return &flux.InvokeError{
				StatusCode: http.StatusTooManyRequests,
				Message:    "RATE:OVER_LIMIT",
			}
		}
	}
}

func (*RateLimitFilter) TypeId() string {
	return FilterIdRateLimitFilter
}