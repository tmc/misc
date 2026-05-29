// Package webidl2plan9 parses a subset of Web IDL and renders it as a Plan 9 /
// Wanix synthetic-filesystem service(4) manpage. It owns the deterministic part
// of the design — the parts the IDL fully settles: which interfaces and
// dictionaries become scopes and files, which attributes are readable status,
// which methods become ctl verbs or clone allocators, file permissions from
// readonly-ness, and the complete feature-mapping table. Design judgment that
// the IDL does not contain (the choice of namespace idiom, fails-closed policy
// for non-crossable types) is applied by documented heuristics, marked as such.
package webidl2plan9

import (
	"fmt"
	"regexp"
	"strings"
)

// IDL is a parsed Web IDL fragment: the definitions relevant to filesystem
// mapping. Only the constructs the renderer needs are modeled.
type IDL struct {
	Interfaces   []*Interface
	Dictionaries []*Dictionary
	Enums        []*Enum
	Callbacks    []*Callback
	Typedefs     []*Typedef
	Includes     map[string][]string // interface name -> included mixin names
}

// Interface is a Web IDL interface: its members and any mixins it includes.
type Interface struct {
	Name       string
	Inherits   string // parent interface, "" if none
	Attributes []*Attribute
	Operations []*Operation
}

// Attribute is an interface attribute. ReadOnly drives the file's permissions.
type Attribute struct {
	Name       string
	Type       string
	ReadOnly   bool
	Deprecated bool // marked **DEPRECATED** in a preceding comment
	Experiment bool // marked **EXPERIMENTAL**
}

// Operation is an interface method. Static, return type, and arguments drive
// whether it becomes a clone allocator, a ctl verb, or a value-returning file.
type Operation struct {
	Name       string
	Return     string
	Args       []Arg
	Static     bool
	Deprecated bool
	Experiment bool
}

// Arg is an operation argument.
type Arg struct {
	Name     string
	Type     string
	Optional bool
}

// Dictionary is a Web IDL dictionary: its members become staged draft files.
type Dictionary struct {
	Name     string
	Inherits string
	Members  []DictMember
}

// DictMember is a dictionary field.
type DictMember struct {
	Name     string
	Type     string
	Required bool
	Default  string
}

// Enum is a Web IDL enum: its values are the legal tokens of a status file.
type Enum struct {
	Name   string
	Values []string
}

// Callback is a Web IDL callback: the source of call/return pipe pairs.
type Callback struct {
	Name   string
	Return string
}

// Typedef is a Web IDL typedef, including union types (the "or" form), which
// tell the renderer which value kinds must be represented or fail closed.
type Typedef struct {
	Name  string
	Type  string   // raw RHS
	Union []string // member types when the RHS is a union
}

var (
	reInterface  = regexp.MustCompile(`^interface\s+(\w+)\s*(?::\s*(\w+))?\s*\{`)
	reDictionary = regexp.MustCompile(`^dictionary\s+(\w+)\s*(?::\s*(\w+))?\s*\{`)
	reEnum       = regexp.MustCompile(`^enum\s+(\w+)\s*\{([^}]*)\}`)
	reCallback   = regexp.MustCompile(`^callback\s+(\w+)\s*=\s*([\w<>?\s]+?)\s*\(`)
	reTypedef    = regexp.MustCompile(`^typedef\s+(.+?)\s+(\w+)\s*;`)
	reIncludes   = regexp.MustCompile(`^(\w+)\s+includes\s+(\w+)\s*;`)
	reAttr       = regexp.MustCompile(`^(readonly\s+)?attribute\s+(.+?)\s+(\w+)\s*;`)
	reEnumVal    = regexp.MustCompile(`"([^"]*)"`)
)

// Parse reads a Web IDL fragment and returns the parsed definitions. It is
// deliberately a subset parser: it understands interfaces, dictionaries, enums,
// callbacks, typedefs (incl. unions), includes, attributes, and operations —
// enough to drive the filesystem mapping — and skips extended attributes
// ([Exposed=...]) and anything it does not recognize rather than failing.
func Parse(src string) (*IDL, error) {
	idl := &IDL{Includes: map[string][]string{}}
	lines := splitLogicalLines(src)

	// Member-level **DEPRECATED**/**EXPERIMENTAL** comments are tracked inside
	// the interface/dict body parsers (where they attach to a member); at the
	// top level such comments are skipped.
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "//") {
			continue
		}
		// strip a leading [extended attribute] block
		if strings.HasPrefix(line, "[") {
			if j := strings.Index(line, "]"); j >= 0 {
				line = strings.TrimSpace(line[j+1:])
			}
			if line == "" {
				continue
			}
		}

		switch {
		case reIncludes.MatchString(line):
			m := reIncludes.FindStringSubmatch(line)
			idl.Includes[m[1]] = append(idl.Includes[m[1]], m[2])
		case reEnum.MatchString(line):
			m := reEnum.FindStringSubmatch(line)
			var vals []string
			for _, v := range reEnumVal.FindAllStringSubmatch(m[2], -1) {
				vals = append(vals, v[1])
			}
			idl.Enums = append(idl.Enums, &Enum{Name: m[1], Values: vals})
		case reCallback.MatchString(line):
			m := reCallback.FindStringSubmatch(line)
			idl.Callbacks = append(idl.Callbacks, &Callback{Name: m[1], Return: strings.TrimSpace(m[2])})
		case reTypedef.MatchString(line):
			m := reTypedef.FindStringSubmatch(line)
			idl.Typedefs = append(idl.Typedefs, &Typedef{Name: m[2], Type: m[1], Union: parseUnion(m[1])})
		case reInterface.MatchString(line):
			m := reInterface.FindStringSubmatch(line)
			iface := &Interface{Name: m[1], Inherits: m[2]}
			i = parseInterfaceBody(lines, i+1, iface)
			idl.Interfaces = append(idl.Interfaces, iface)
		case reDictionary.MatchString(line):
			m := reDictionary.FindStringSubmatch(line)
			d := &Dictionary{Name: m[1], Inherits: m[2]}
			i = parseDictBody(lines, i+1, d)
			idl.Dictionaries = append(idl.Dictionaries, d)
		default:
			// unrecognized top-level line; ignore
		}
	}
	if len(idl.Interfaces) == 0 && len(idl.Dictionaries) == 0 {
		return nil, fmt.Errorf("no interfaces or dictionaries found; is this Web IDL?")
	}
	return idl, nil
}

// splitLogicalLines collapses multi-line operation signatures (args spanning
// lines until the closing ';') into one logical line each, so the member
// parsers can work line-at-a-time. Comment lines are preserved on their own.
func splitLogicalLines(src string) []string {
	var out []string
	var buf strings.Builder
	depth := 0
	for _, raw := range strings.Split(src, "\n") {
		t := strings.TrimSpace(raw)
		if depth == 0 && (t == "" || strings.HasPrefix(t, "//")) {
			out = append(out, raw)
			continue
		}
		buf.WriteString(" ")
		buf.WriteString(t)
		depth += strings.Count(t, "(") - strings.Count(t, ")")
		// a statement ends at ';' or a brace when not inside arg parens
		if depth <= 0 && (strings.HasSuffix(t, ";") || strings.HasSuffix(t, "{") || strings.HasSuffix(t, "}")) {
			out = append(out, strings.TrimSpace(buf.String()))
			buf.Reset()
			depth = 0
		}
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		out = append(out, s)
	}
	return out
}

var (
	reOp = regexp.MustCompile(`^(static\s+)?(.+?)\s+(\w+)\s*\((.*)\)\s*;`)
)

func parseInterfaceBody(lines []string, start int, iface *Interface) int {
	var dep, exp bool
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "}" || line == "};" {
			return i
		}
		if strings.HasPrefix(line, "//") {
			if strings.Contains(line, "**DEPRECATED**") {
				dep = true
			}
			if strings.Contains(line, "**EXPERIMENTAL**") {
				exp = true
			}
			continue
		}
		if m := reAttr.FindStringSubmatch(line); m != nil {
			iface.Attributes = append(iface.Attributes, &Attribute{
				Name: m[3], Type: strings.TrimSpace(m[2]),
				ReadOnly: m[1] != "", Deprecated: dep, Experiment: exp,
			})
			dep, exp = false, false
			continue
		}
		if m := reOp.FindStringSubmatch(line); m != nil {
			op := &Operation{
				Name: m[3], Return: strings.TrimSpace(m[2]),
				Static: m[1] != "", Args: parseArgs(m[4]),
				Deprecated: dep, Experiment: exp,
			}
			iface.Operations = append(iface.Operations, op)
			dep, exp = false, false
			continue
		}
		dep, exp = false, false
	}
	return len(lines) - 1
}

var reDictMember = regexp.MustCompile(`^(required\s+)?(.+?)\s+(\w+)\s*(?:=\s*(.+?))?\s*;`)

func parseDictBody(lines []string, start int, d *Dictionary) int {
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "}" || line == "};" {
			return i
		}
		if strings.HasPrefix(line, "//") {
			continue
		}
		if m := reDictMember.FindStringSubmatch(line); m != nil {
			d.Members = append(d.Members, DictMember{
				Name: m[3], Type: strings.TrimSpace(m[2]),
				Required: m[1] != "", Default: strings.TrimSpace(m[4]),
			})
		}
	}
	return len(lines) - 1
}

func parseArgs(s string) []Arg {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var args []Arg
	for _, part := range splitTopLevel(s, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		opt := false
		if strings.HasPrefix(part, "optional ") {
			opt = true
			part = strings.TrimSpace(strings.TrimPrefix(part, "optional "))
		}
		// drop a default value
		if eq := strings.Index(part, "="); eq >= 0 {
			part = strings.TrimSpace(part[:eq])
		}
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		args = append(args, Arg{Name: fields[len(fields)-1], Type: strings.Join(fields[:len(fields)-1], " "), Optional: opt})
	}
	return args
}

// splitTopLevel splits on sep but not inside <> or () — so a union or generic
// argument type is kept whole.
func splitTopLevel(s string, sep byte) []string {
	var out []string
	var buf strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '<', '(':
			depth++
		case '>', ')':
			depth--
		}
		if c == sep && depth == 0 {
			out = append(out, buf.String())
			buf.Reset()
			continue
		}
		buf.WriteByte(c)
	}
	out = append(out, buf.String())
	return out
}

// parseUnion returns the member types of a "(A or B or C)" union, or nil.
func parseUnion(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		return nil
	}
	inner := s[1 : len(s)-1]
	var members []string
	for _, m := range strings.Split(inner, " or ") {
		if m = strings.TrimSpace(m); m != "" {
			members = append(members, m)
		}
	}
	if len(members) < 2 {
		return nil
	}
	return members
}
