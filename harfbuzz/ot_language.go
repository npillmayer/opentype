package harfbuzz

import (
	"github.com/go-text/typesetting/font/opentype/tables"
)

type langTag struct {
	language string
	tag      tables.Tag
}
