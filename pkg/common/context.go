package common

import (
	"context"
	"github.com/bytepowered/fluxgo/pkg/internal"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
)

import (
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/listener"
)

var _slim = echo.New()

// NewSlimContextTODO 创建SlimContext对象。SlimContext不可用于具有写入响应的场景。
func NewSlimContextTODO(id string) flux.Context {
	return NewSlimContext(context.TODO(), id)
}

// NewSlimContext 创建SlimContext对象。SlimContext不可用于具有写入响应的场景。
func NewSlimContext(ctx context.Context, id string, vars ...map[string]interface{}) flux.Context {
	fxctx := internal.NewContext()
	fxctx.Reset(newSlimWithID(ctx, id), &flux.EndpointSpec{Application: "slim"})
	fxctx.SetVariable("flux.go/is.slim.ctx", true)
	if len(vars) > 0 {
		for k, v := range vars[0] {
			fxctx.SetVariable(k, v)
		}
	}
	return fxctx
}

func newSlimWithID(ctx context.Context, id string) flux.WebContext {
	req := httptest.NewRequest("GET", "http://slimctx/"+id, nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	return listener.NewWebContext(_slim.NewContext(req, rec), id, nil)
}
