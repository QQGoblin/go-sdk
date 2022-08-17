package http

import "github.com/pkg/errors"

var ErrorInvalidRouterPattern = errors.New("invalid router pattern")
var ErrorInvalidMethod = errors.New("invalid method")
var ErrorHookTimeout = errors.New("the hook timeout")
