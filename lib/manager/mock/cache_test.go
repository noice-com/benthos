package mock_test

import (
	"github.com/Jeffail/benthos/v3/lib/manager/mock"
	"github.com/Jeffail/benthos/v3/lib/types"
)

var _ types.Cache = &mock.Cache{}
