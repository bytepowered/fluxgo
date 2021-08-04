package ext

import (
	"fmt"
	"github.com/bytepowered/fluxgo/pkg/flux"
	assert2 "github.com/stretchr/testify/assert"
	"testing"
)

var (
	_ flux.Filter  = new(TestOrderedFilter)
	_ flux.Filter  = new(TestFilter)
	_ flux.Orderer = new(TestOrderedFilter)
)

type TestOrderedFilter struct {
	order int
}

func (f *TestOrderedFilter) FilterId() string {
	return fmt.Sprintf("%d", f.order)
}

func (f *TestOrderedFilter) DoFilter(_ flux.FilterInvoker) flux.FilterInvoker {
	return func(context flux.Context) *flux.ServeError {
		return nil
	}
}

func (f *TestOrderedFilter) Order() int {
	return f.order
}

type TestFilter struct {
	id string
}

func (f *TestFilter) FilterId() string {
	return f.id
}

func (f *TestFilter) DoFilter(_ flux.FilterInvoker) flux.FilterInvoker {
	return func(context flux.Context) *flux.ServeError {
		return nil
	}
}

func TestFilterArrayOrder(t *testing.T) {
	filters := []interface{}{
		&TestOrderedFilter{order: 2},
		&TestOrderedFilter{order: 3},
		&TestOrderedFilter{order: 1},
		&TestOrderedFilter{order: 5},
		&TestOrderedFilter{order: 4},
	}
	for _, f := range filters {
		AddGlobalFilter(f)
		AddSelectiveFilter(f)
	}
	shouldBeOrder := []int{1, 2, 3, 4, 5}
	globals := GlobalFilters()
	selective := SelectiveFilters()
	assert := assert2.New(t)
	assert.Equal(len(filters), len(globals))
	assert.Equal(len(filters), len(selective))
	for i, f := range globals {
		order := f.(*TestOrderedFilter).order
		assert.Equal(shouldBeOrder[i], order)
	}
	for i, f := range selective {
		order := f.(*TestOrderedFilter).order
		assert.Equal(shouldBeOrder[i], order)
	}
}

func TestFilterArrayMixedOrder(t *testing.T) {
	filters := []interface{}{
		&TestOrderedFilter{order: 2},
		&TestOrderedFilter{order: 3},
		&TestOrderedFilter{order: 1},
		&TestOrderedFilter{order: 5},
		&TestFilter{id: "TF001"},
		&TestFilter{id: "TF002"},
		&TestOrderedFilter{order: 4},
	}
	for _, f := range filters {
		AddGlobalFilter(f)
		AddSelectiveFilter(f)
	}
	assert := assert2.New(t)
	f0 := GlobalFilters()[0]
	f1 := SelectiveFilters()[1]
	assert.Equal("TF001", f0.FilterId())
	assert.Equal("TF002", f1.FilterId())
	s0, ok := SelectiveFilterById("TF002")
	assert.Equal(true, ok)
	assert.Equal("TF002", s0.FilterId())
}
