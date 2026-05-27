// Package applefs defines host-backed Apple capability file servers.
package applefs

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrNotExist   = errors.New("file does not exist")
	ErrPermission = errors.New("permission denied")
	ErrBadCtl     = errors.New("bad ctl")
)

type Entry struct {
	Name string
	Dir  bool
}

type Host interface {
	Apply(name string, data []byte) error
}

type Root struct {
	mu       sync.Mutex
	services map[string]*Service
	aliases  map[string]string
	Host     Host
	OnWrite  func(name string, data []byte)
	OnAppend func(name string, data []byte)
}

func NewRoot() *Root {
	r := &Root{
		services: make(map[string]*Service),
		aliases: map[string]string{
			"app":        "appkit/app",
			"window":     "appkit/window",
			"pasteboard": "appkit/pasteboard",
			"alert":      "appkit/alert",
		},
	}
	for _, svc := range BuiltinServices() {
		r.Mount(svc)
	}
	return r
}

func (r *Root) Mount(s *Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[s.Name] = s
}

func (r *Root) ReadDir(name string) ([]Entry, error) {
	name = clean(name)
	if name == "." {
		r.mu.Lock()
		defer r.mu.Unlock()
		var out []Entry
		for name := range r.services {
			out = append(out, Entry{Name: name, Dir: true})
		}
		for name := range r.aliases {
			out = append(out, Entry{Name: name, Dir: true})
		}
		out = append(out, Entry{Name: "status", Dir: false})
		sortEntries(out)
		return out, nil
	}
	svc, rest, err := r.lookup(name)
	if err != nil {
		return nil, err
	}
	return svc.ReadDir(rest)
}

func (r *Root) ReadFile(name string) ([]byte, error) {
	if clean(name) == "status" {
		return []byte("api macos\nstatus ok\n"), nil
	}
	svc, rest, err := r.lookup(name)
	if err != nil {
		return nil, err
	}
	return svc.ReadFile(rest)
}

func (r *Root) WriteFile(name string, data []byte) error {
	if clean(name) == "status" {
		return pathError("write", name, ErrPermission)
	}
	svc, rest, err := r.lookup(name)
	if err != nil {
		return err
	}
	if err := svc.WriteFile(rest, data); err != nil {
		return err
	}
	if r.OnWrite != nil {
		r.OnWrite(name, data)
		for _, alias := range r.aliasNames(name) {
			r.OnWrite(alias, data)
		}
	}
	if r.Host != nil {
		return r.Host.Apply(canonicalName(svc.Name, rest), data)
	}
	return nil
}

func (r *Root) AppendFile(name string, data []byte) error {
	if clean(name) == "status" {
		return pathError("write", name, ErrPermission)
	}
	svc, rest, err := r.lookup(name)
	if err != nil {
		return err
	}
	if err := svc.AppendFile(rest, data); err != nil {
		return err
	}
	if r.OnAppend != nil {
		r.OnAppend(name, data)
		for _, alias := range r.aliasNames(name) {
			r.OnAppend(alias, data)
		}
	}
	return nil
}

func (r *Root) EnsureClonePath(name string) bool {
	svc, rest, err := r.lookup(name)
	if err != nil {
		return false
	}
	return svc.EnsureClonePath(rest)
}

func (r *Root) CloneSchemas() map[string]map[string]string {
	r.mu.Lock()
	services := make([]*Service, 0, len(r.services))
	for _, svc := range r.services {
		services = append(services, svc)
	}
	aliases := make(map[string]string, len(r.aliases))
	for alias, target := range r.aliases {
		aliases[alias] = target
	}
	r.mu.Unlock()
	out := make(map[string]map[string]string)
	for _, svc := range services {
		for name, files := range svc.CloneSchemas() {
			key := svc.Name
			if name != "." {
				key += "/" + name
			}
			out[key] = files
		}
	}
	aliasSchemas := make(map[string]map[string]string)
	for alias, target := range aliases {
		for key, files := range out {
			if key == target || strings.HasPrefix(key, target+"/") {
				aliasSchemas[alias+strings.TrimPrefix(key, target)] = files
			}
		}
	}
	for key, files := range aliasSchemas {
		out[key] = files
	}
	return out
}

func canonicalName(service, rest string) string {
	if rest == "." || rest == "" {
		return service
	}
	return service + "/" + rest
}

func (r *Root) aliasNames(name string) []string {
	name = clean(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for alias, target := range r.aliases {
		if name == target || strings.HasPrefix(name, target+"/") {
			out = append(out, alias+strings.TrimPrefix(name, target))
		}
	}
	sort.Strings(out)
	return out
}

func (r *Root) lookup(name string) (*Service, string, error) {
	name = clean(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	for alias, target := range r.aliases {
		if name == alias || strings.HasPrefix(name, alias+"/") {
			name = target + strings.TrimPrefix(name, alias)
			break
		}
	}
	head, rest, _ := strings.Cut(name, "/")
	svc := r.services[head]
	if svc == nil {
		return nil, "", pathError("open", name, ErrNotExist)
	}
	if rest == "" {
		rest = "."
	}
	return svc, rest, nil
}

type Service struct {
	Name string

	mu       sync.Mutex
	dirs     map[string]bool
	files    map[string]File
	clones   map[string]*Clone
	readonly bool
}

func NewService(name string) *Service {
	s := &Service{
		Name:   name,
		dirs:   map[string]bool{".": true},
		files:  make(map[string]File),
		clones: make(map[string]*Clone),
	}
	return s
}

func (s *Service) ReadDir(name string) ([]Entry, error) {
	name = clean(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirs[name] {
		return nil, pathError("readdir", name, ErrNotExist)
	}
	children := make(map[string]Entry)
	for dir := range s.dirs {
		addChild(children, name, dir, true)
	}
	for file := range s.files {
		addChild(children, name, file, false)
	}
	var out []Entry
	for _, e := range children {
		out = append(out, e)
	}
	sortEntries(out)
	return out, nil
}

func (s *Service) ReadFile(name string) ([]byte, error) {
	name = clean(name)
	s.mu.Lock()
	f := s.files[name]
	s.mu.Unlock()
	if f == nil {
		return nil, pathError("read", name, ErrNotExist)
	}
	return f.Read()
}

func (s *Service) WriteFile(name string, data []byte) error {
	name = clean(name)
	s.mu.Lock()
	f := s.files[name]
	readonly := s.readonly
	s.mu.Unlock()
	if f == nil {
		return pathError("write", name, ErrNotExist)
	}
	if readonly {
		return pathError("write", name, ErrPermission)
	}
	return f.Write(data)
}

func (s *Service) AppendFile(name string, data []byte) error {
	name = clean(name)
	s.mu.Lock()
	f := s.files[name]
	readonly := s.readonly
	s.mu.Unlock()
	if f == nil {
		return pathError("write", name, ErrNotExist)
	}
	if readonly {
		return pathError("write", name, ErrPermission)
	}
	if f, ok := f.(interface{ Append([]byte) error }); ok {
		return f.Append(data)
	}
	old, err := f.Read()
	if err != nil {
		return err
	}
	next := make([]byte, 0, len(old)+len(data))
	next = append(next, old...)
	next = append(next, data...)
	return f.Write(next)
}

func (s *Service) EnsureClonePath(name string) bool {
	name = clean(name)
	s.mu.Lock()
	var clones []*Clone
	for _, clone := range s.clones {
		clones = append(clones, clone)
	}
	s.mu.Unlock()
	for _, clone := range clones {
		if id, ok := clone.sessionID(name); ok {
			clone.ensure(id)
			return true
		}
	}
	return false
}

func (s *Service) CloneSchemas() map[string]map[string]string {
	s.mu.Lock()
	clones := make(map[string]*Clone, len(s.clones))
	for name, clone := range s.clones {
		clones[name] = clone
	}
	s.mu.Unlock()
	out := make(map[string]map[string]string)
	for name, clone := range clones {
		prefix := strings.TrimSuffix(name, "/clone")
		if name == "clone" {
			prefix = "."
		}
		files := clone.files("0")
		schema := make(map[string]string, len(files))
		for file, f := range files {
			data, err := f.Read()
			if err != nil {
				data = nil
			}
			schema[file] = string(data)
		}
		out[prefix] = schema
	}
	return out
}

func (s *Service) Dir(name string) *Service {
	name = clean(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addDirLocked(name)
	return s
}

func (s *Service) File(name string, f File) *Service {
	name = clean(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if dir := path.Dir(name); dir != "." {
		s.addDirLocked(dir)
	}
	s.files[name] = f
	return s
}

func (s *Service) Clone(name string, files func(id string) map[string]File) *Service {
	name = clean(name)
	prefix := strings.TrimSuffix(name, "/clone")
	if name == "clone" {
		prefix = "."
	}
	c := &Clone{service: s, prefix: prefix, files: files}
	s.clones[name] = c
	s.File(name, c)
	return s
}

func (s *Service) ReadOnly() *Service {
	s.readonly = true
	return s
}

func (s *Service) addDirLocked(name string) {
	name = clean(name)
	if name == "." {
		s.dirs[name] = true
		return
	}
	var cur string
	for _, elem := range strings.Split(name, "/") {
		if cur == "" {
			cur = elem
		} else {
			cur += "/" + elem
		}
		s.dirs[cur] = true
	}
}

type File interface {
	Read() ([]byte, error)
	Write([]byte) error
}

type TextFile struct {
	mu sync.Mutex
	s  string
}

func NewTextFile(s string) *TextFile { return &TextFile{s: s} }

func (f *TextFile) Read() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return []byte(f.s), nil
}

func (f *TextFile) Write(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.s = string(data)
	return nil
}

func (f *TextFile) Append(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.s += string(data)
	return nil
}

type ReadFile string

func (f ReadFile) Read() ([]byte, error) { return []byte(f), nil }
func (f ReadFile) Write([]byte) error    { return ErrPermission }

type StreamFile struct {
	mu  sync.Mutex
	buf []byte
}

func NewStreamFile(data []byte) *StreamFile {
	f := &StreamFile{}
	_ = f.Write(data)
	return f
}

func (f *StreamFile) Read() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]byte, len(f.buf))
	copy(out, f.buf)
	f.buf = f.buf[:0]
	return out, nil
}

func (f *StreamFile) Write(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buf = append(f.buf[:0], data...)
	return nil
}

func (f *StreamFile) Append(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buf = append(f.buf, data...)
	return nil
}

type CtlFile struct {
	Verb func(string) error
}

func (f CtlFile) Read() ([]byte, error) { return nil, ErrPermission }
func (f CtlFile) Write(data []byte) error {
	verb := strings.TrimSpace(string(data))
	if f.Verb == nil {
		return pathError("ctl", verb, ErrBadCtl)
	}
	return f.Verb(verb)
}

type Clone struct {
	mu      sync.Mutex
	next    int
	service *Service
	prefix  string
	files   func(id string) map[string]File
}

func (c *Clone) Read() ([]byte, error) {
	c.mu.Lock()
	c.next++
	id := fmt.Sprint(c.next)
	c.mu.Unlock()

	c.ensure(id)
	return []byte(id + "\n"), nil
}

func (c *Clone) sessionID(name string) (string, bool) {
	name = clean(name)
	if c.prefix == "." {
		id, _, _ := strings.Cut(name, "/")
		return id, id != "" && decimal(id)
	}
	if name == c.prefix {
		return "", false
	}
	rest := strings.TrimPrefix(name, c.prefix+"/")
	if rest == name {
		return "", false
	}
	id, _, _ := strings.Cut(rest, "/")
	return id, id != "" && decimal(id)
}

func (c *Clone) ensure(id string) {
	if n, err := strconv.Atoi(id); err == nil {
		c.mu.Lock()
		if n > c.next {
			c.next = n
		}
		c.mu.Unlock()
	}

	c.service.mu.Lock()
	defer c.service.mu.Unlock()
	dir := id
	if c.prefix != "." {
		dir = c.prefix + "/" + id
	}
	if c.service.dirs[dir] {
		return
	}
	c.service.addDirLocked(dir)
	for name, file := range c.files(id) {
		c.service.files[dir+"/"+name] = file
	}
}

func (c *Clone) Write([]byte) error { return ErrPermission }

func decimal(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func clean(name string) string {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if name == "" {
		return "."
	}
	return name
}

func addChild(children map[string]Entry, parent, child string, dir bool) {
	if child == parent {
		return
	}
	var rest string
	if parent == "." {
		rest = child
	} else {
		if !strings.HasPrefix(child, parent+"/") {
			return
		}
		rest = strings.TrimPrefix(child, parent+"/")
	}
	if rest == "" || strings.Contains(rest, "/") {
		return
	}
	children[rest] = Entry{Name: rest, Dir: dir}
}

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return entries[i].Name < entries[j].Name
	})
}

func pathError(op, name string, err error) error {
	return fmt.Errorf("%s %s: %w", op, name, err)
}
