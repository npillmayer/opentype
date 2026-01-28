package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/npillmayer/opentype"
	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/schuko/schukonf/testconfig"
	"github.com/npillmayer/schuko/tracing"
	"github.com/npillmayer/schuko/tracing/gologadapter"
	"github.com/npillmayer/schuko/tracing/trace2go"
	"github.com/pterm/pterm"
)

// tracer traces with key 'tyse.fonts'
func tracer() tracing.Trace {
	return tracing.Select("tyse.fonts")
}

func main() {
	initDisplay()

	// set up logging
	tracing.RegisterTraceAdapter("go", gologadapter.GetAdapter(), false)
	conf := testconfig.Conf{
		"tracing.adapter":  "go",
		"trace.tyse.fonts": "Info",
	}
	if err := trace2go.ConfigureRoot(conf, "trace", trace2go.ReplaceTracers(true)); err != nil {
		fmt.Printf("error configuring tracing")
		os.Exit(1)
	}
	tracing.SetTraceSelector(trace2go.Selector())

	// command line flags
	tlevel := flag.String("trace", "Info", "Trace level [Debug|Info|Error]")
	fontname := flag.String("font", "", "Font to load")
	flag.Parse()
	tracer().SetTraceLevel(tracing.LevelError)    // will set the correct level later
	pterm.Info.Println("Welcome to OpenType CLI") // colored welcome message
	//
	// set up REPL
	repl, err := readline.New("ot > ")
	if err != nil {
		tracer().Errorf(err.Error())
		os.Exit(3)
	}
	intp := &Intp{repl: repl, stack: make([]pathNode, 0, 100)}
	//
	// load font to use
	if err := intp.loadFont(*fontname); err != nil { // font name provided by flag
		tracer().Errorf(err.Error())
		os.Exit(4)
	}
	//
	// start receiving commands
	pterm.Info.Println("Quit with <ctrl>D") // inform user how to stop the CLI
	switch *tlevel {
	case "Debug":
		tracer().SetTraceLevel(tracing.LevelDebug)
	case "Info":
		tracer().SetTraceLevel(tracing.LevelInfo)
	case "Error":
		tracer().SetTraceLevel(tracing.LevelError)
	default:
		tracer().Errorf("Invalid trace level: %s", *tlevel)
		os.Exit(5)
	}
	tracer().Infof("Trace level is %s", *tlevel)
	intp.REPL() // go into interactive mode
}

// We use pterm for moderately fancy output.
func initDisplay() {
	pterm.EnableDebugMessages()
	pterm.Info.Prefix = pterm.Prefix{
		Text:  " !  ",
		Style: pterm.NewStyle(pterm.BgCyan, pterm.FgBlack),
	}
	pterm.Error.Prefix = pterm.Prefix{
		Text:  " Error",
		Style: pterm.NewStyle(pterm.BgRed, pterm.FgBlack),
	}
}

type pathNode struct {
	table    ot.Table
	location ot.Navigator
	link     ot.NavLink
	key      string
	inx      int
}

func (n *pathNode) String() string {
	if n == nil || n.location == nil {
		return "<none>"
	}
	s := n.location.Name()
	if n.key != "" {
		s += fmt.Sprintf("[%s]", n.key)
	} else if n.inx >= 0 {
		s += fmt.Sprintf("[%d]", n.inx)
	}
	if n.link != nil {
		s += " -> (" + n.link.Name() + ")"
	}
	return s
}

// Intp is our interpreter object
type Intp struct {
	font  *ot.Font
	repl  *readline.Instance
	table ot.Table
	stack []pathNode
}

func (intp *Intp) String() string {
	if intp == nil || intp.table == nil {
		return "()"
	}
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("( table=%s )", intp.table.Self().NameTag()))
	for _, node := range intp.stack {
		sb.WriteString(fmt.Sprintf(" -> %s", node.String()))
	}
	return sb.String()
}

// REPL starts interactive mode.
func (intp *Intp) REPL() {
	for {
		//tracer().Infof(intp.String())
		pterm.Println(intp.String())
		line, err := intp.repl.Readline()
		if err != nil { // io.EOF
			break
		}
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		println(line)
		cmd, err := intp.parseCommand(line)
		if err != nil {
			tracer().Errorf(err.Error())
			continue
		}
		err, quit := intp.execute(cmd)
		if err != nil {
			tracer().Errorf(err.Error())
			continue
		}
		if quit {
			break
		}
	}
	pterm.Info.Println("Good bye!")
}

type Op struct {
	code   int
	arg    string
	format string
}

type Command struct {
	count int
	op    [32]Op
}

const NOOP = -1
const (
	// op-codes QUIT and NAVIGATE will not have arguments
	QUIT int = iota
	NAVIGATE
	// op-codes below may have arguments
	HELP
	TABLE
	LIST
	MAP
	SCRIPTS
	FEATURES
	LOOKUPS
	PRINT
)

var opMap = map[string]int{
	"quit":     QUIT,
	"->":       NAVIGATE,
	"help":     HELP,
	"table":    TABLE,
	"map":      MAP,
	"list":     LIST,
	"scripts":  SCRIPTS,
	"features": FEATURES,
	"lookups":  LOOKUPS,
	"print":    PRINT,
}

var opNames = []string{
	"quit",
	"->",
	"help",
	"table",
	"map",
	"list",
	"scripts",
	"features",
	"lookups",
	"print",
}

var command = Command{}

func resetCommand() {
	command.count = 0
	for i := range command.op {
		command.op[i].code = NOOP
		command.op[i].arg = ""
		command.op[i].format = ""
	}
}

func (intp *Intp) parseCommand(line string) (*Command, error) {
	resetCommand()
	steps := strings.Split(line, " ")
	command.count = len(steps)
	for i, step := range steps {
		c := strings.Split(step, ":") // e.g.  "scripts:latn:tag" or "list:5:int" or "help:lang" or "map"
		code, ok := opMap[strings.ToLower(c[0])]
		if !ok {
			code = HELP
		}
		command.op[i].code = code
		command.op[i].arg = ""
		if command.op[i].code <= NAVIGATE {
			return &command, nil
		}
		tracer().Infof("parsed command: %v", c)
		command.op[i].arg = getOptArg(c, 1)
		command.op[i].format = getOptArg(c, 2)
		if command.op[i].arg == "" {
			tracer().Infof("%s", opNames[command.op[i].code])
		} else {
			//command.op[i].arg = strings.ToLower(command.op[i].arg)
			tracer().Infof("%s: looking for '%s'", opNames[command.op[i].code], command.op[i].arg)
		}
	}
	return &command, nil
}

var commandFn = map[int]func(*Intp, *Op) (error, bool){
	QUIT:     quitOp,
	NAVIGATE: navigateOp,
	HELP:     helpOp,
	TABLE:    tableOp,
	LIST:     listOp,
	MAP:      mapOp,
	SCRIPTS:  scriptsOp,
	FEATURES: featuresOp,
	LOOKUPS:  lookupsOp,
	PRINT:    printOp,
}

func (intp *Intp) execute(cmd *Command) (err error, stop bool) {
	tracer().Debugf("cmd = %v", cmd.op)
	for _, c := range cmd.op {
		if c.code == NOOP {
			break
		}
		f, ok := commandFn[c.code]
		if !ok {
			pterm.Error.Printf("unknown command code: %d\n", c.code)
			return nil, false
		}
		err, stop = f(intp, &c)
		if err != nil {
			pterm.Error.Println(err)
			return
		}
		if stop {
			return
		}
	}
	return
}

// func notimpl(intp *Intp, op *Op) (error, bool) {
// 	return errors.New("not implemented"), false
// }

func quitOp(intp *Intp, op *Op) (error, bool) {
	pterm.Println("Goodbye!")
	return nil, true
}

func navigateOp(intp *Intp, op *Op) (error, bool) {
	if intp.table == nil {
		pterm.Error.Println("cannot walk without table being set")
	} else if intp.table == intp.lastPathNode().table {
		tracer().Infof("ignoring '->'")
	} else if intp.lastPathNode().link == nil {
		pterm.Error.Println("no link to walk")
	} else {
		l := intp.lastPathNode().link
		n := pathNode{location: l.Navigate(), inx: -1}
		intp.stack = append(intp.stack, n)
		tracer().Infof("walked to %s", n.location.Name())
	}
	return nil, false
}

// --- Font Loading -----------------------------------------------------

// Test function to load a font from a file
func (intp *Intp) loadFont(fontname string) (err error) {
	intp.font, err = loadLocalFont(fontname)
	if err == nil {
		pterm.Printf("font tables: %v\n", intp.font.TableTags())
	}
	return
}

func loadLocalFont(fontFileName string) (*ot.Font, error) {
	path := filepath.Join("..", "testdata", fontFileName)
	f, err := opentype.LoadOpenTypeFont(path)
	if err != nil {
		tracer().Errorf("cannot load test font %s: %s", fontFileName, err)
		return nil, err
	}
	tracer().Infof("loaded SFNT font = %s", f.Fontname)
	otf, err := ot.Parse(f.Binary)
	if err != nil {
		tracer().Errorf("cannot decode test font %s: %s", fontFileName, err)
		return nil, err
	}
	otf.F = f
	tracer().Infof("parsed OpenType font = %s", otf.F.Fontname)
	return otf, nil
}

// ----------------------------------------------------------------------

func (intp *Intp) lastPathNode() pathNode {
	if len(intp.stack) == 0 {
		return pathNode{}
	}
	return intp.stack[len(intp.stack)-1]
}

func (intp *Intp) setLastPathNode(n pathNode) {
	if len(intp.stack) == 0 {
		intp.stack = append(intp.stack, n)
	}
	intp.stack[len(intp.stack)-1] = n
}

func (intp *Intp) clearPath() ot.Table {
	intp.stack = intp.stack[:0]
	return intp.table
}

// TODO
func decodeLocation(loc ot.NavLocation, name string) interface{} {
	if loc == nil {
		return nil
	}
	switch loc.Size() {
	case 2:
		return int(loc.U16(0))
	case 4:
		return int(loc.U32(0))
	default:
		switch name {
		case "FeatureRecord":
			tag := ot.Tag(loc.U32(0))
			link := int(loc.U16(4))
			return struct {
				ot.Tag
				int
			}{tag, link}
		}
	}
	return nil
}

var ERR_NO_TABLE = errors.New("no table set")
var ERR_NO_LOCATION = errors.New("no location set")
var ERR_VOID = errors.New("location is void")

func (intp *Intp) checkTable() error {
	if intp.table == nil {
		return ERR_NO_TABLE
	}
	return nil
}

func (intp *Intp) checkLocation() (loc ot.Navigator, err error) {
	loc = intp.lastPathNode().location
	if err = intp.checkTable(); err != nil {
		return
	} else if loc == nil {
		err = ERR_NO_LOCATION
	} else if loc.IsVoid() {
		err = ERR_VOID
	}
	return
}

func getOptArg(s []string, inx int) string {
	if len(s) > inx {
		return s[inx]
	}
	return ""
}

func (op *Op) noArg() bool {
	if op.arg == "" {
		return true
	}
	return false
}

func (op *Op) hasArg() (string, bool) {
	if op.arg == "" {
		return "", false
	}
	return op.arg, true
}
