# OpenType CLI

An interactive command-line tool for inspecting and exploring OpenType font structures.

## Overview

`otcli` is a REPL (Read-Eval-Print Loop) that provides interactive access to OpenType font internals. It's designed for:

- **Font developers** - Inspect and debug OpenType layout tables
- **Typography researchers** - Explore font features and their implementations  
- **Library development** - Test and validate the opentype library's navigation patterns
- **Learning** - Understand OpenType font structure through hands-on exploration

The tool leverages the "Schrödinger's cat" navigation pattern from the `ot` package, allowing lazy, on-demand traversal of font structures without loading everything into memory upfront.

## Features

### Interactive Exploration

- **Table Navigation** - Browse any OpenType table (GSUB, GPOS, GDEF, cmap, etc.)
- **Command Chaining** - Combine multiple navigation steps in a single line
- **Navigation Stack** - Track your path through nested font structures
- **Pretty Output** - Color-coded display with formatted tables and lists
- **Built-in Help** - Context-sensitive help for OpenType structures

### Supported Commands

| Command | Description | Example |
|---------|-------------|---------|
| `table:<tag>` | Load a specific font table | `table:GSUB` |
| `map[:<key>]` | Navigate map structures, lookup by tag | `map:latn` |
| `list[:<index>]` | Navigate list structures, get item | `list:5` |
| `scripts[:<tag>]` | Access ScriptList, navigate to script | `scripts:arab` |
| `features[:<index>]` | Access FeatureList, get feature | `features:10` |
| `lookups[:<index>]` | Print GSUB LookupList or a specific lookup | `lookups` |
| `->` | Follow the current navigation link | `->` |
| `help[:<topic>]` | Show help (topics: script, lang) | `help:script` |
| `quit` | Exit the REPL | `quit` |

### Command Chaining

Commands can be chained with spaces to perform complex navigation in one line:

```
table:GSUB scripts:latn -> map:TRK -> list:0
```

This navigates: GSUB table → ScriptList → Latin script → Turkish language → first feature

## Installation

### Prerequisites

- Go 1.19 or later
- Access to the opentype library repository

### Build from Source

```bash
cd otcli
go build -o otcli main.go
```

### Run Directly

```bash
cd otcli
go run main.go -font <fontfile> -trace <level>
```

## Usage

### Basic Usage

```bash
# Run with a font file
./otcli -font ../testdata/Calibri.ttf

# Set trace level for debugging
./otcli -font ../testdata/Calibri.ttf -trace Debug
```

### Command Line Options

- `-font <filename>` - Font file to load (required, relative to testdata directory)
- `-trace <level>` - Trace level: Debug, Info, or Error (default: Info)

### Interactive Session Example

```
$ ./otcli -font Calibri.ttf -trace Info

Welcome to OpenType CLI
Quit with <ctrl>D

ot > table:GSUB
font tables: [GDEF GPOS GSUB OS/2 cmap cvt fpgm glyf ...]

ot > scripts
ScriptList keys: [DFLT arab armn beng cher cyrl ...]

ot > scripts:latn -> map
Script table maps [latn] = Script

ot > map
Script map keys = [DFLT AZE  CRT  KAZ  MOL  ROM  TAT  TRK  ]

ot > map:TRK -> list
List has 24 entries

ot > list:0
LangSys list index 0 holds number = 3

ot > help:script
 !   ScriptList / Script

	ScriptList is a property of GSUB and GPOS.
	It consists of ScriptRecords:
	+------------+----------------+
	| Script Tag | Link to Script |
	+------------+----------------+
	ScriptList behaves as a map.
	...

ot > quit
Good bye!
```

## Use Cases

### Debugging Font Features

Explore how a specific font implements OpenType features:

```
ot > table:GSUB
ot > features:5
GSUB list index 5 holds feature record = liga

ot > scripts:latn -> map:dflt -> list
List has 15 entries  # 15 features for Latin default
```

### Understanding Script Support

Check which scripts and languages a font supports:

```
ot > table:GSUB
ot > scripts
ScriptList keys: [DFLT arab latn ...]

ot > scripts:arab -> map
Script map keys = [DFLT ARA  FAR  URD  ]
```

### Validating Parsing

Verify that the library correctly parses complex table structures:

```
ot > table:GPOS
ot > scripts:arab -> map:FAR -> list:3
# Inspect specific lookup indices for Farsi language
```

### Learning OpenType Layout

Interactively explore the hierarchical structure of layout tables:

```
# Follow the path: Table → ScriptList → Script → LangSys → Features
ot > table:GSUB scripts:latn -> map:TRK -> list
```

## Architecture

### Navigation Model

The tool maintains a navigation stack similar to a file system:

```go
type pathNode struct {
    table    ot.Table      // Current table
    location ot.Navigator  // Current position
    link     ot.NavLink    // Link to follow with ->
}
```

Each command either:
- **Selects** a new table (resets the stack)
- **Navigates** to a new location (adds to stack)
- **Queries** the current location (reads from stack)

### Output Formatting

Uses [pterm](https://github.com/pterm/pterm) for colored, formatted output:
- **Blue info messages** - System information
- **Red error messages** - Error conditions  
- **Plain text** - Data and query results

## Development

### Adding New Commands

To add a new command:

1. Add command constant in `main.go`:
   ```go
   const NEWCMD int = iota
   ```

2. Add parsing logic in `parseCommand()`:
   ```go
   case "newcmd":
       command.op[i].code = NEWCMD
   ```

3. Add execution logic in `execute()`:
   ```go
   case NEWCMD:
       // Implementation
   ```

### Testing

The CLI tool is excellent for testing new features in the `ot` package:

1. Make changes to the `ot` package
2. Run `otcli` with a test font
3. Navigate to the affected structures
4. Verify the output matches expectations

## Dependencies

- [github.com/chzyer/readline](https://github.com/chzyer/readline) - Line editing with history
- [github.com/pterm/pterm](https://github.com/pterm/pterm) - Pretty terminal output
- github.com/npillmayer/schuko/tracing - Trace logging
- github.com/npillmayer/tyse/core/font - Font file loading

## Limitations

- **Read-only** - Cannot modify font structures
- **No export** - Cannot save modified fonts (by design)
- **Single font** - Load one font at a time
- **Limited help** - Help system needs expansion for more topics

## Future Enhancements

Potential improvements:

- [ ] More help topics (features, lookups, coverage)
- [ ] Export navigation results to JSON
- [ ] Compare two fonts side-by-side
- [ ] Visualize glyph coverage ranges
- [ ] Navigate GDEF, BASE, and other tables
- [ ] History search (readline already supports this)
- [ ] Tab completion for table/script/feature tags

## License

Governed by a 3-Clause BSD license. License file may be found in the root folder of this module.

Copyright © Norbert Pillmayer <norbert@pillmayer.com>

## See Also

- [OpenType Specification](https://docs.microsoft.com/en-us/typography/opentype/spec/)
- [Parent Repository](../) - The opentype Go library
- [ot Package](../ot/) - Low-level OpenType parsing
- [otquery Package](../otquery/) - High-level font queries
- [otlayout Package](../otlayout/) - Layout feature application
