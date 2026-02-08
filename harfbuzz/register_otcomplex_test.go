package harfbuzz_test

import (
	"errors"
	"fmt"

	"github.com/npillmayer/opentype/harfbuzz"
	"github.com/npillmayer/opentype/harfbuzz/otcomplex"
)

func init() {
	if err := otcomplex.Register(); err != nil && !errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
		panic(fmt.Sprintf("register otcomplex test shapers: %v", err))
	}
}
