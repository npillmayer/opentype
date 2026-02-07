package harfbuzz_test

import (
	"errors"
	"fmt"

	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/harfbuzz/otcomplex"
)

func init() {
	if err := otcomplex.Register(); err != nil && !errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
		panic(fmt.Sprintf("register otcomplex test shapers: %v", err))
	}
}
