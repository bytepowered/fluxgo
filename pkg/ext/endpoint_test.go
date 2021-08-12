package ext

import (
	assert2 "github.com/stretchr/testify/assert"
	"testing"
)

func TestMakeEndpointSpecKey(t *testing.T) {
	assert := assert2.New(t)
	assert.Equal("GET#/api/user/+", MakeEndpointSpecKey("GET", "/api/user/{uid}"))
	assert.Equal("GET#/api/user/+/", MakeEndpointSpecKey("GET", "/api/user/{uid}/"))
	assert.Equal("GET#/api/user/+/profile", MakeEndpointSpecKey("GET", "/api/user/{uid}/profile"))
	assert.Equal("GET#/api/user/+/profile/+", MakeEndpointSpecKey("GET", "/api/user/{uid}/profile/{pid}"))
	assert.Equal("GET#/api/user/+/profile/+", MakeEndpointSpecKey("GET", "/api/user/{ uid }/profile/{pid  }"))
}
