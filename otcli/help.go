package main

import (
	"strings"

	"github.com/pterm/pterm"
)

func helpOp(intp *Intp, op *Op) (error, bool) {
	help(op.arg)
	return nil, false
}

func help(topic string) {
	tracer().Infof("help %v", topic)
	t := strings.ToLower(topic)
	switch t {
	case "script", "scripts", "scriptList":
		pterm.Info.Println("ScriptList / Script")
		pterm.Println(`
	ScriptList is a property of GSUB and GPOS.
	It consists of ScriptRecords:
	+------------+----------------+
	| Script Tag | Link to Script |
	+------------+----------------+
	ScriptList behaves as a map.

	A Script table links to a default LangSys entry, and contains a list of LangSys records:
	+--------------------------------+
	| Link to LangSys record         |
	+--------------+-----------------+
	| Language Tag | Link to LangSys |
	+--------------+-----------------+
	Script behaves as a map, with entry 0 as the default link
	`)
	case "lang", "langsys", "langs", "language":
		pterm.Info.Println("LangSys")
		pterm.Println(`
	LangSys is pointed to from a Script Record.
	It links a language with features to activate. It does so using an index into the feature table.
	+-----------------------------------+
	| Index of required feature or null |
	+-----------------------------------+
	| Index of feature 1                |
	+-----------------------------------+
	| Index of feature 2                |
	+-----------------------------------+
	| ...                               |
	+-----------------------------------+
	LangSys behaves as a list.
	`)
	default:
		pterm.Info.Println("General Help, TODO")
	}
}
