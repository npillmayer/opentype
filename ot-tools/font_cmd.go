package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/npillmayer/opentype/ot"
	"github.com/npillmayer/opentype/otquery"
	"github.com/thatisuday/commando"
)

func runFontCommand(args map[string]commando.ArgValue, flags map[string]commando.FlagValue) {
	fontPath := strings.TrimSpace(args["font"].Value)
	if fontPath == "" {
		fatalf("font path is required")
	}
	otf := mustLoadFont(fontPath, mustFlagBool(flags["testfont"], "testfont"))

	fmt.Printf("Path: %s\n", fontPath)
	fmt.Printf("Type: %s\n", otquery.FontType(otf))
	names := otquery.NameInfo(otf, 0)
	if family := names["family"]; family != "" {
		fmt.Printf("Family: %s\n", family)
	}
	if sub := names["subfamily"]; sub != "" {
		fmt.Printf("Subfamily: %s\n", sub)
	}
	if version := names["version"]; version != "" {
		fmt.Printf("Version: %s\n", version)
	}

	tags := otf.TableTags()
	sort.Slice(tags, func(i, j int) bool { return tags[i] < tags[j] })
	fmt.Printf("Tables (%d):", len(tags))
	for _, tag := range tags {
		fmt.Printf(" %s", tag.String())
	}
	fmt.Println()

	layoutTables := otquery.LayoutTables(otf)
	sort.Strings(layoutTables)
	fmt.Printf("Layout: %s\n", strings.Join(layoutTables, ","))

	errs := otf.Errors()
	warns := otf.Warnings()
	crit := otf.CriticalErrors()
	fmt.Printf("Issues: errors=%d warnings=%d critical=%d\n", len(errs), len(warns), len(crit))

	if len(args["tables"].Value) > 0 {
		printSelectedTables(otf, args["tables"].Value)
	}
	showIssues, err := flags["errors"].GetBool()
	if err != nil {
		fatalf("invalid --errors flag: %v", err)
	}
	if showIssues {
		for _, e := range errs {
			fmt.Printf("error: %s\n", e.Error())
		}
		for _, w := range warns {
			fmt.Printf("warning: %s\n", w.String())
		}
	}
}

func printSelectedTables(otf *ot.Font, raw string) {
	requested := splitCSVSpace(raw)
	for _, t := range requested {
		tagName := strings.TrimSpace(t)
		if tagName == "" {
			continue
		}
		tag := ot.T(tagName)
		table := otf.Table(tag)
		if table == nil {
			fmt.Printf("table %s: missing\n", tagName)
			continue
		}
		off, size := table.Extent()
		fmt.Printf("table %s: offset=%d size=%d\n", tagName, off, size)
	}
}
