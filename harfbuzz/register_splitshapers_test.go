package harfbuzz_test

import (
	"errors"
	"fmt"

	"github.com/npillmayer/opentype/harfbuzz"
	"github.com/npillmayer/opentype/harfbuzz/otarabic"
	"github.com/npillmayer/opentype/harfbuzz/othebrew"
)

func init() {
	if err := othebrew.Register(); err != nil && !errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
		panic(fmt.Sprintf("register othebrew test shaper: %v", err))
	}
	if err := otarabic.Register(); err != nil && !errors.Is(err, harfbuzz.ErrShaperAlreadyRegistered) {
		panic(fmt.Sprintf("register otarabic test shaper: %v", err))
	}
}
