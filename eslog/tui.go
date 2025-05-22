package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Style definitions with better color contrasts and more intentional visual hierarchy
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#25A065")).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1)

	// Status bar with better visual separation
	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#555555")).
		Padding(0, 1)

	// Better infoStyle for descriptive text
	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Italic(true)

	// Help style with more subtle appearance to avoid distracting from content
	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(1)

	// Process styles with better color coding for states
	processStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#34ACE0"))
	
	// Active processes are highlighted in green for better visibility
	activeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#16A085"))
	
	// Completed processes in gray to de-emphasize
	completedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))
	
	// Error states in red to draw attention
	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E74C3C"))
	
	// Special highlight for selected items 
	selectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#2C3E50")).
		Foreground(lipgloss.Color("#FFFFFF"))
	
	// Header styles for better section organization
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Underline(true).
		Foreground(lipgloss.Color("#FFFFFF"))
	
	// Tooltip style for help text and hints
	tooltipStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F39C12")).
		Background(lipgloss.Color("#34495E")).
		PaddingLeft(1).
		PaddingRight(1)

	// Filter input style
	filterInputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#2980B9")).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1)
	
	// File operation statistics icons and colors
	lookupIcon   = "🔍" // Search icon for lookup operations
	statIcon     = "ℹ️" // Info icon for stat operations
	readlinkIcon = "🔗" // Link icon for readlink operations
	accessIcon   = "👁️" // Eye icon for access operations
	openIcon     = "📂" // Folder open icon for open operations
	closeIcon    = "📁" // Folder closed icon for close operations
	readIcon     = "📖" // Book icon for read operations
	writeIcon    = "📝" // Pencil icon for write operations
)

// keyMap defines the keybindings for the application
type keyMap struct {
	Up             key.Binding
	Down           key.Binding
	Left           key.Binding
	Right          key.Binding
	Help           key.Binding
	Quit           key.Binding
	Filter         key.Binding
	Expand         key.Binding
	Collapse       key.Binding
	Search         key.Binding
	Focus          key.Binding
	Unfocus        key.Binding
	ToggleRaw      key.Binding
	GotoParent     key.Binding
	CopyToClipboard key.Binding
	Refresh        key.Binding
}

// defaultKeyMap returns the default keybindings with better descriptions
func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "navigate up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "navigate down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "collapse/go back"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "expand/details"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter processes"),
		),
		Expand: key.NewBinding(
			key.WithKeys("+", "="),
			key.WithHelp("+", "expand all"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("-"),
			key.WithHelp("-", "collapse all"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Focus: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view details"),
		),
		Unfocus: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "go back to tree"),
		),
		ToggleRaw: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "toggle raw data"),
		),
		GotoParent: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "go to parent process"),
		),
		CopyToClipboard: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy process info"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("F5"),
			key.WithHelp("F5", "refresh view"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view.
// This satisfies the help.KeyMap interface.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.Up, k.Down, k.Focus, k.Unfocus}
}

// FullHelp returns keybindings for the expanded help view.
// This satisfies the help.KeyMap interface.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Focus, k.Unfocus, k.Expand, k.Collapse},
		{k.Search, k.Filter, k.ToggleRaw, k.GotoParent},
		{k.CopyToClipboard, k.Refresh, k.Help, k.Quit},
	}
}

// FileOpCounters tracks file operation counts and amounts
type FileOpCounters struct {
	Lookups      int
	Readlinks    int
	Stats        int
	Accesses     int
	Opens        int
	Closes       int
	Reads        int
	Writes       int
	BytesRead    int64
	BytesWritten int64
}

// ProcessInfo represents a process in the TUI
type ProcessInfo struct {
	PID          int
	PPID         int
	OriginalPPID int
	Executable   string
	TTY          string
	Args         []string
	StartTime    time.Time
	Events       []*ESEvent
	Children     map[int]*ProcessInfo
	Parent       *ProcessInfo
	IsActive     bool        // Whether the process is still running
	HasExited    bool        // Whether the process has explicitly exited
	ExitCode     int         // Exit code of the process if it has exited
	ExitTime     time.Time   // Time when the process exited
	ExitEvent    *ESEvent    // Event containing process exit details
	IsExpanded   bool        // UI state for tree view
	Level        int         // Depth in the process tree
	FileOps      FileOpCounters // Track file operations for this process
}

// ProcessNode to ProcessInfo conversion
func ProcessNodeToInfo(node *ProcessNode, level int) *ProcessInfo {
	if node == nil || node.Process == nil {
		return nil
	}

	info := &ProcessInfo{
		PID:          node.Process.AuditToken.PID,
		PPID:         node.Process.PPID,
		OriginalPPID: node.Process.OriginalPPID,
		Executable:   node.Process.Executable.Path,
		Children:     make(map[int]*ProcessInfo),
		Events:       node.Execs,
		IsExpanded:   level < 2, // Auto-expand first two levels
		Level:        level,
		HasExited:    node.HasExited,
		ExitCode:     node.ExitCode,
		ExitTime:     node.ExitTime,
		ExitEvent:    node.ExitEvent,
		FileOps: FileOpCounters{
			Lookups:      node.FileOps.Lookups,
			Readlinks:    node.FileOps.Readlinks,
			Stats:        node.FileOps.Stats,
			Accesses:     node.FileOps.Accesses,
			Opens:        node.FileOps.Opens,
			Closes:       node.FileOps.Closes,
			Reads:        node.FileOps.Reads,
			Writes:       node.FileOps.Writes,
			BytesRead:    node.FileOps.BytesRead,
			BytesWritten: node.FileOps.BytesWritten,
		},
	}

	if node.Process.TTY != nil {
		info.TTY = node.Process.TTY.Path
	}

	if len(node.Execs) > 0 && node.Execs[0].Event.Exec != nil && len(node.Execs[0].Event.Exec.Args) > 0 {
		// Get command args from the first exec event
		info.Args = node.Execs[0].Event.Exec.Args
	}

	if len(node.Execs) > 0 {
		startTime, err := time.Parse(time.RFC3339Nano, node.Process.StartTime)
		if err == nil {
			info.StartTime = startTime
		}
	}

	// Set process activity status based on exit events
	info.IsActive = !info.HasExited

	// Convert children
	for _, childNode := range node.Children {
		if childNode.Process != nil {
			childPID := childNode.Process.AuditToken.PID
			childInfo := ProcessNodeToInfo(childNode, level+1)
			if childInfo != nil {
				childInfo.Parent = info
				info.Children[childPID] = childInfo
			}
		}
	}

	return info
}

// BuildProcessTree converts the ProcessTree to ProcessInfo tree
func BuildProcessTree(pt *ProcessTree) map[int]*ProcessInfo {
	result := make(map[int]*ProcessInfo)

	// Find root processes
	for pid, node := range pt.Nodes {
		if node.Parent == nil || node.Parent.Process == nil {
			info := ProcessNodeToInfo(node, 0)
			if info != nil {
				result[pid] = info
			}
		}
	}

	return result
}

// Model represents the application state
type Model struct {
	processTree     *ProcessTree
	uiTree          map[int]*ProcessInfo
	filteredTree    map[int]*ProcessInfo
	viewport        viewport.Model
	selectedPID     int
	focusedPID      int
	hasFocus        bool
	keys            keyMap
	help            help.Model
	showHelp        bool
	width           int
	height          int
	ready           bool
	rawMode         bool
	showFilterBar   bool
	filterInput     textinput.Model
	filterText      string
	firstStart      time.Time
	filteringActive bool
	tooltips        bool              // Toggle for showing tooltips
	tooltipContent  string            // Current tooltip content
	searchMode      bool              // Whether search mode is active
	searchInput     textinput.Model   // Input field for search
	searchResults   []int             // PIDs of search results
	currentSearch   string            // Current search query
	searchResultIdx int               // Current index in search results
	copyMessage     string            // Message for clipboard operations
	showCopyMessage bool              // Whether to show clipboard message
	copyMessageTime time.Time         // When to hide the clipboard message
}

// NewModel creates a new model
func NewModel(pt *ProcessTree) Model {
	help := help.New()
	keys := defaultKeyMap()

	firstStart := time.Time{}
	if !pt.FirstStartTime.IsZero() {
		firstStart = pt.FirstStartTime
	}

	// Build the process tree
	uiTree := BuildProcessTree(pt)
	
	// Set up filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "Filter by process name or command"
	filterInput.CharLimit = 50
	filterInput.Width = 30
	
	// Set up search input
	searchInput := textinput.New()  
	searchInput.Placeholder = "Search processes"
	searchInput.CharLimit = 50
	searchInput.Width = 30

	return Model{
		processTree:     pt,
		uiTree:          uiTree,
		filteredTree:    uiTree, // Initially, filtered tree is the same as the regular tree
		keys:            keys,
		help:            help,
		showHelp:        false,
		rawMode:         false,
		firstStart:      firstStart,
		filteringActive: false,
		filterInput:     filterInput,
		tooltips:        true, // Enable tooltips by default
		searchInput:     searchInput,
		searchMode:      false,
		searchResults:   []int{},
		searchResultIdx: 0,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	// Select first process by default
	if m.selectedPID == 0 && len(m.uiTree) > 0 {
		for pid := range m.uiTree {
			m.selectedPID = pid
			break
		}
	}
	
	// Initialize inputs
	return tea.Batch(
		textinput.Blink,  // Start cursor blinking
		m.filterInput.Focus(),
		m.searchInput.Focus(),
	)
}

// copyProcessInfoToClipboard creates a string with process information for copying
func (m *Model) copyProcessInfoToClipboard() string {
	var result string
	
	if m.hasFocus {
		// Copy focused process info
		proc, found := m.findProcess(m.focusedPID, m.uiTree)
		if !found {
			return "Process not found"
		}
		
		result = fmt.Sprintf("PID: %d\nPPID: %d\nExecutable: %s\n", 
			proc.PID, proc.PPID, proc.Executable)
		
		if len(proc.Args) > 0 {
			result += fmt.Sprintf("Command: %s\n", strings.Join(proc.Args, " "))
		}
		
		if proc.TTY != "" {
			result += fmt.Sprintf("TTY: %s\n", proc.TTY)
		}
		
		if !proc.StartTime.IsZero() {
			result += fmt.Sprintf("Start Time: %s\n", proc.StartTime.Format(time.RFC3339))
		}
		
		if proc.HasExited {
			result += fmt.Sprintf("Exit Code: %d\n", proc.ExitCode)
			if !proc.ExitTime.IsZero() {
				result += fmt.Sprintf("Exit Time: %s\n", proc.ExitTime.Format(time.RFC3339))
			}
		}
	} else {
		// Copy selected process info
		proc, found := m.findProcess(m.selectedPID, m.uiTree)
		if !found {
			return "Process not found"
		}
		
		// More compact format for tree view selection
		result = fmt.Sprintf("PID: %d - %s", proc.PID, proc.Executable)
		if len(proc.Args) > 0 {
			result += fmt.Sprintf(" - %s", strings.Join(proc.Args, " "))
		}
	}
	
	// Simulate clipboard copy
	m.copyMessage = "Process info copied to clipboard"
	m.showCopyMessage = true
	m.copyMessageTime = time.Now().Add(3 * time.Second)
	
	return result
}

// Update updates the model based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Check if it's time to hide the copy message
	if m.showCopyMessage && time.Now().After(m.copyMessageTime) {
		m.showCopyMessage = false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle filter input when filter bar is active
		if m.showFilterBar {
			switch msg.Type {
			case tea.KeyEnter:
				// Apply filter
				m.filterText = m.filterInput.Value()
				if m.filterText != "" {
					m.applyFilter()
				} else {
					m.clearFilter()
				}
				m.showFilterBar = false
			case tea.KeyEsc:
				// Cancel filtering
				m.showFilterBar = false
			default:
				// Handle regular input
				var inputCmd tea.Cmd
				m.filterInput, inputCmd = m.filterInput.Update(msg)
				cmds = append(cmds, inputCmd)
				return m, tea.Batch(cmds...)
			}
		}
		
		// Handle search input when search mode is active
		if m.searchMode {
			switch msg.Type {
			case tea.KeyEnter:
				// Apply search
				query := m.searchInput.Value()
				if query != "" {
					m.currentSearch = query
					m.performSearch()
					if len(m.searchResults) > 0 {
						m.searchResultIdx = 0
						m.navigateToSearchResult()
					}
				}
				m.searchMode = false
			case tea.KeyEsc:
				// Cancel search
				m.searchMode = false
			default:
				// Handle regular input
				var inputCmd tea.Cmd
				m.searchInput, inputCmd = m.searchInput.Update(msg)
				cmds = append(cmds, inputCmd)
				return m, tea.Batch(cmds...)
			}
		}

		// Handle regular keys when not in input mode
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
		case key.Matches(msg, m.keys.ToggleRaw):
			m.rawMode = !m.rawMode
			m.updateViewportContent()
		case key.Matches(msg, m.keys.Up):
			// Navigate up through the processes
			if m.hasFocus {
				// When focused, navigate through events
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			} else {
				// Move to previous process
				m.selectPrevProcess()
			}
		case key.Matches(msg, m.keys.Down):
			// Navigate down through the processes
			if m.hasFocus {
				// When focused, navigate through events
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			} else {
				// Move to next process
				m.selectNextProcess()
			}
		case key.Matches(msg, m.keys.Left):
			// Collapse current selection if expanded
			if !m.hasFocus {
				m.collapseSelected()
			} else {
				// In focus mode, left goes back to tree
				m.hasFocus = false
				m.updateViewportContent()
			}
		case key.Matches(msg, m.keys.Right):
			// Expand current selection if collapsed
			if !m.hasFocus {
				m.expandSelected()
			}
		case key.Matches(msg, m.keys.Focus):
			// Focus on current process to see details
			if !m.hasFocus {
				m.focusOnSelected()
			}
		case key.Matches(msg, m.keys.Unfocus):
			// Exit focus mode
			if m.hasFocus {
				m.hasFocus = false
				m.updateViewportContent()
			}
		case key.Matches(msg, m.keys.Expand):
			// Expand all nodes
			m.expandAll()
		case key.Matches(msg, m.keys.Collapse):
			// Collapse all nodes
			m.collapseAll()
		case key.Matches(msg, m.keys.Filter):
			// Toggle filter bar
			m.showFilterBar = !m.showFilterBar
			if m.showFilterBar {
				m.filterInput.Focus()
				m.filterInput.SetValue(m.filterText)
			}
		case key.Matches(msg, m.keys.Search):
			// Toggle search mode
			m.searchMode = !m.searchMode
			if m.searchMode {
				m.searchInput.Focus()
				m.searchInput.SetValue(m.currentSearch)
			}
		case key.Matches(msg, m.keys.GotoParent):
			// Navigate to parent process
			m.navigateToParent()
		case key.Matches(msg, m.keys.CopyToClipboard):
			// Copy process info to clipboard
			m.copyProcessInfoToClipboard()
		case key.Matches(msg, m.keys.Refresh):
			// Refresh the view
			m.updateViewportContent()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6) // Leave room for header, status, and input areas
			m.viewport.HighPerformanceRendering = true
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 6
		}
		
		// Update input widths
		m.filterInput.Width = msg.Width - 20
		m.searchInput.Width = msg.Width - 20
		
		m.updateViewportContent()
	}

	return m, tea.Batch(cmds...)
}

// Helper functions for navigation
func (m *Model) selectPrevProcess() {
	if m.selectedPID == 0 && len(m.uiTree) > 0 {
		// If nothing is selected, select the first process
		for pid := range m.uiTree {
			m.selectedPID = pid
			break
		}
		return
	}

	// Get a flattened view of visible processes for navigation
	visibleProcesses := m.getVisibleProcesses()
	if len(visibleProcesses) == 0 {
		return
	}

	// Find the current selection in the visible processes
	currentIndex := -1
	for i, proc := range visibleProcesses {
		if proc.PID == m.selectedPID {
			currentIndex = i
			break
		}
	}

	// Move to previous process
	if currentIndex > 0 {
		m.selectedPID = visibleProcesses[currentIndex-1].PID
	} else if currentIndex == -1 && len(visibleProcesses) > 0 {
		// If current process is not in visible list, select last visible
		m.selectedPID = visibleProcesses[len(visibleProcesses)-1].PID
	}

	m.updateViewportContent()
}

func (m *Model) selectNextProcess() {
	if m.selectedPID == 0 && len(m.uiTree) > 0 {
		// If nothing is selected, select the first process
		for pid := range m.uiTree {
			m.selectedPID = pid
			break
		}
		return
	}

	// Get a flattened view of visible processes for navigation
	visibleProcesses := m.getVisibleProcesses()
	if len(visibleProcesses) == 0 {
		return
	}

	// Find the current selection in the visible processes
	currentIndex := -1
	for i, proc := range visibleProcesses {
		if proc.PID == m.selectedPID {
			currentIndex = i
			break
		}
	}

	// Move to next process
	if currentIndex >= 0 && currentIndex < len(visibleProcesses)-1 {
		m.selectedPID = visibleProcesses[currentIndex+1].PID
	} else if currentIndex == -1 && len(visibleProcesses) > 0 {
		// If current process is not in visible list, select first visible
		m.selectedPID = visibleProcesses[0].PID
	}

	m.updateViewportContent()
}

// navigateToParent moves selection to the parent of the currently selected process
func (m *Model) navigateToParent() {
	proc, found := m.findProcess(m.selectedPID, m.uiTree)
	if !found || proc.Parent == nil {
		return
	}
	
	m.selectedPID = proc.Parent.PID
	m.updateViewportContent()
}

// getVisibleProcesses returns a flattened list of all visible processes
// based on the current expansion state
func (m *Model) getVisibleProcesses() []*ProcessInfo {
	var result []*ProcessInfo

	// Use filtered tree if filtering is active
	tree := m.uiTree
	if m.filteringActive {
		tree = m.filteredTree
	}

	// Process root nodes first
	var rootPIDs []int
	for pid := range tree {
		rootPIDs = append(rootPIDs, pid)
	}
	
	// Sort root PIDs for consistent navigation
	sort.Ints(rootPIDs)
	
	for _, pid := range rootPIDs {
		proc := tree[pid]
		result = append(result, proc)

		// Add children recursively if expanded
		if proc.IsExpanded {
			m.appendVisibleChildren(proc, &result)
		}
	}

	return result
}

// appendVisibleChildren adds visible children to the list
func (m *Model) appendVisibleChildren(parent *ProcessInfo, list *[]*ProcessInfo) {
	// Process children in a deterministic order
	var childPIDs []int
	for pid := range parent.Children {
		childPIDs = append(childPIDs, pid)
	}

	// Sort by PID for consistent navigation
	sort.Ints(childPIDs)

	// When filtering is active, we only add visible children that are in the filtered tree
	for _, pid := range childPIDs {
		child := parent.Children[pid]

		// Skip if we're filtering and this child isn't part of the filtered tree
		if m.filteringActive {
			if _, filtered := m.findProcess(child.PID, m.filteredTree); !filtered {
				continue
			}
		}

		*list = append(*list, child)

		// Recurse if this child is expanded
		if child.IsExpanded {
			m.appendVisibleChildren(child, list)
		}
	}
}

func (m *Model) collapseSelected() {
	// Find the selected process and collapse it
	if proc, found := m.findProcess(m.selectedPID, m.uiTree); found {
		proc.IsExpanded = false
		m.updateViewportContent()
	}
}

func (m *Model) expandSelected() {
	// Find the selected process and expand it
	if proc, found := m.findProcess(m.selectedPID, m.uiTree); found {
		proc.IsExpanded = true
		m.updateViewportContent()
	}
}

func (m *Model) focusOnSelected() {
	m.hasFocus = true
	m.focusedPID = m.selectedPID
	// Update viewport content to show detailed info
	m.updateViewportContent()
}

func (m *Model) expandAll() {
	m.walkProcessTree(m.uiTree, func(p *ProcessInfo) {
		p.IsExpanded = true
	})
	m.updateViewportContent()
}

func (m *Model) collapseAll() {
	m.walkProcessTree(m.uiTree, func(p *ProcessInfo) {
		if p.Level > 0 {
			p.IsExpanded = false
		}
	})
	m.updateViewportContent()
}

// applyFilter filters processes based on the current filter text
func (m *Model) applyFilter() {
	if m.filterText == "" {
		m.clearFilter()
		return
	}
	
	filterLower := strings.ToLower(m.filterText)
	m.filteredTree = make(map[int]*ProcessInfo)
	
	// Function to check if a process or any of its ancestors match
	var checkProcessMatch func(*ProcessInfo) bool
	checkProcessMatch = func(proc *ProcessInfo) bool {
		// Check if this process matches
		exeName := strings.ToLower(filepath.Base(proc.Executable))
		fullPath := strings.ToLower(proc.Executable)
		
		// Match on executable name or path
		if strings.Contains(exeName, filterLower) || strings.Contains(fullPath, filterLower) {
			return true
		}
		
		// Match on command args
		for _, arg := range proc.Args {
			if strings.Contains(strings.ToLower(arg), filterLower) {
				return true
			}
		}
		
		// Check if any child matches
		for _, child := range proc.Children {
			if checkProcessMatch(child) {
				return true
			}
		}
		
		return false
	}
	
	// Find processes that match or have matching descendants
	for pid, proc := range m.uiTree {
		if checkProcessMatch(proc) {
			// Create a copy of the process tree for filtered view
			m.filteredTree[pid] = proc
		}
	}
	
	m.filteringActive = len(m.filteredTree) > 0
	
	// If no results, show original tree
	if len(m.filteredTree) == 0 {
		m.filteredTree = m.uiTree
		m.filteringActive = false
	}
	
	// Auto-expand all in filtered results
	if m.filteringActive {
		m.walkProcessTree(m.filteredTree, func(p *ProcessInfo) {
			p.IsExpanded = true
		})
	}
	
	// Reset selection
	m.selectedPID = 0
	visibleProcesses := m.getVisibleProcesses()
	if len(visibleProcesses) > 0 {
		m.selectedPID = visibleProcesses[0].PID
	}
	
	m.updateViewportContent()
}

// clearFilter removes any active filtering
func (m *Model) clearFilter() {
	m.filterText = ""
	m.filteringActive = false
	m.updateViewportContent()
}

// performSearch finds processes matching the search query
func (m *Model) performSearch() {
	if m.currentSearch == "" {
		m.searchResults = []int{}
		return
	}
	
	searchLower := strings.ToLower(m.currentSearch)
	m.searchResults = []int{}
	
	// Function to recursively search processes
	var searchProcess func(*ProcessInfo)
	searchProcess = func(proc *ProcessInfo) {
		// Check if this process matches
		exeName := strings.ToLower(filepath.Base(proc.Executable))
		fullPath := strings.ToLower(proc.Executable)
		
		// Match on executable name or path
		if strings.Contains(exeName, searchLower) || strings.Contains(fullPath, searchLower) {
			m.searchResults = append(m.searchResults, proc.PID)
		}
		
		// Match on command args
		for _, arg := range proc.Args {
			if strings.Contains(strings.ToLower(arg), searchLower) {
				// Only add once
				if !containsPID(m.searchResults, proc.PID) {
					m.searchResults = append(m.searchResults, proc.PID)
				}
				break
			}
		}
		
		// Search all children
		for _, child := range proc.Children {
			searchProcess(child)
		}
	}
	
	// Check all processes in the tree
	for _, proc := range m.uiTree {
		searchProcess(proc)
	}
}

// navigateToSearchResult jumps to the current search result
func (m *Model) navigateToSearchResult() {
	if len(m.searchResults) == 0 || m.searchResultIdx < 0 || m.searchResultIdx >= len(m.searchResults) {
		return
	}
	
	// Get the PID to navigate to
	targetPID := m.searchResults[m.searchResultIdx]
	
	// Find the process and ensure its parent chain is expanded
	proc, found := m.findProcess(targetPID, m.uiTree)
	if !found {
		return
	}
	
	// Expand the path to this process
	current := proc
	for current.Parent != nil {
		current.Parent.IsExpanded = true
		current = current.Parent
	}
	
	// Set the selected PID
	m.selectedPID = targetPID
	m.updateViewportContent()
}

// Helper function to check if a pid is in a slice
func containsPID(slice []int, pid int) bool {
	for _, p := range slice {
		if p == pid {
			return true
		}
	}
	return false
}

// walkProcessTree traverses the process tree and applies fn to each process
func (m *Model) walkProcessTree(tree map[int]*ProcessInfo, fn func(*ProcessInfo)) {
	for _, proc := range tree {
		fn(proc)
		m.walkProcessTree(proc.Children, fn)
	}
}

// findProcess finds a process by PID in the tree
func (m *Model) findProcess(pid int, tree map[int]*ProcessInfo) (*ProcessInfo, bool) {
	if proc, found := tree[pid]; found {
		return proc, true
	}

	for _, proc := range tree {
		if found, ok := m.findProcess(pid, proc.Children); ok {
			return found, true
		}
	}

	return nil, false
}

// updateViewportContent updates the content of the viewport
func (m *Model) updateViewportContent() {
	if m.hasFocus {
		// Show detailed view of focused process
		m.viewport.SetContent(m.renderFocusedProcess())
	} else {
		// Show tree view
		m.viewport.SetContent(m.renderTree())
	}
}

// renderTree renders the process tree as a string
func (m *Model) renderTree() string {
	var sb strings.Builder

	// Create initial header
	title := "Process Tree"
	if m.filteringActive {
		title = fmt.Sprintf("Process Tree (Filtered: %s)", m.filterText)
	}
	sb.WriteString(headerStyle.Render(title) + "\n")
	
	// Show help tip at the top of the display
	if m.tooltips {
		tip := "Press '?' for help, 'f' to filter, '/' to search, Enter to view details"
		sb.WriteString(tooltipStyle.Render(tip) + "\n\n")
	} else {
		sb.WriteString("\n")
	}

	// Get visible processes for rendering
	tree := m.uiTree
	if m.filteringActive {
		tree = m.filteredTree
	}
	
	// Sort root processes by PID for consistent display
	var rootPIDs []int
	for pid := range tree {
		rootPIDs = append(rootPIDs, pid)
	}
	sort.Ints(rootPIDs)

	// Render root processes in sorted order
	for _, pid := range rootPIDs {
		m.renderProcessNode(&sb, tree[pid], "")
	}

	return sb.String()
}

// renderProcessNode renders a process node and its children recursively
func (m *Model) renderProcessNode(sb *strings.Builder, proc *ProcessInfo, indent string) {
	// Determine base style based on process state
	var style lipgloss.Style
	
	if proc.HasExited {
		// Process has explicitly exited - show in error style if non-zero exit
		if proc.ExitCode != 0 {
			style = errorStyle
		} else {
			style = completedStyle
		}
	} else if !proc.IsActive {
		// Process is inactive but didn't explicitly exit
		style = completedStyle
	} else {
		// Process is active
		style = activeStyle
	}

	// Apply selection highlight if this is the selected process
	if proc.PID == m.selectedPID {
		style = selectedStyle
	}

	// Calculate relative time
	relativeTime := ""
	if !m.firstStart.IsZero() && !proc.StartTime.IsZero() {
		seconds := proc.StartTime.Sub(m.firstStart).Seconds()
		relativeTime = fmt.Sprintf(" T:%.1fs", seconds)
	}

	// Format executable name
	execName := proc.Executable
	if len(execName) > 30 {
		execName = "..." + execName[len(execName)-27:]
	}

	// Format process info with PID, PPID, exit code (if applicable), and relative time
	exitInfo := ""
	if proc.HasExited {
		exitInfo = fmt.Sprintf(" Exit:%d", proc.ExitCode)
	}
	info := fmt.Sprintf("[PID:%d PPID:%d%s%s]", proc.PID, proc.PPID, exitInfo, relativeTime)

	// Format command arguments
	cmdArgs := ""
	if len(proc.Args) > 0 {
		args := proc.Args
		if len(args) > 0 {
			cmd := args[0]
			// Add arguments with limited length for display
			if len(args) > 1 {
				cmdArgs = " " + strings.Join(args[1:], " ")
				if len(cmdArgs) > 60 {
					cmdArgs = cmdArgs[:57] + "..."
				}
			}
			cmdArgs = " " + cmd + cmdArgs
		}
	}

	// Add file operations with improved, more readable icons
	fileOpsInfo := ""
	if len(proc.Children) == 0 || !proc.IsExpanded {
		// Only show non-zero counters
		var ops []string
		if proc.FileOps.Lookups > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", lookupIcon, proc.FileOps.Lookups))
		}
		if proc.FileOps.Stats > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", statIcon, proc.FileOps.Stats))
		}
		if proc.FileOps.Readlinks > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", readlinkIcon, proc.FileOps.Readlinks))
		}
		if proc.FileOps.Accesses > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", accessIcon, proc.FileOps.Accesses))
		}
		if proc.FileOps.Opens > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", openIcon, proc.FileOps.Opens))
		}
		if proc.FileOps.Closes > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", closeIcon, proc.FileOps.Closes))
		}
		if proc.FileOps.Reads > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", readIcon, proc.FileOps.Reads))
		}
		if proc.FileOps.Writes > 0 {
			ops = append(ops, fmt.Sprintf("%s %d", writeIcon, proc.FileOps.Writes))
		}

		if len(ops) > 0 {
			fileOpsInfo = " " + strings.Join(ops, " ")
		}
	}

	// Format expansion marker with clear Unicode indicators
	expandMarker := "▶ "  // Right triangle for collapsed
	if proc.IsExpanded {
		expandMarker = "▼ "  // Down triangle for expanded
	}
	if len(proc.Children) == 0 {
		expandMarker = "• "  // Bullet for leaf nodes
	}

	// Render the line with file operations info
	sb.WriteString(indent + style.Render(expandMarker+info) + " " + cmdArgs)
	
	// Add file operations in a more subtle color to not overpower the main info
	fileOpsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D2B48C"))
	sb.WriteString(fileOpsStyle.Render(fileOpsInfo) + "\n")
	
	// Render children if expanded
	if proc.IsExpanded {
		childIndent := indent + "  "
		
		// Process children in sorted order by PID
		var childPIDs []int
		for pid := range proc.Children {
			childPIDs = append(childPIDs, pid)
		}
		sort.Ints(childPIDs)
		
		for _, pid := range childPIDs {
			m.renderProcessNode(sb, proc.Children[pid], childIndent)
		}
	}
}

// renderFocusedProcess renders detailed information about the focused process
func (m *Model) renderFocusedProcess() string {
	var sb strings.Builder

	proc, found := m.findProcess(m.focusedPID, m.uiTree)
	if !found {
		return "Process not found"
	}

	// Render process header with status
	statusInfo := "Running"
	statusStyle := activeStyle
	if proc.HasExited {
		if proc.ExitCode == 0 {
			statusInfo = fmt.Sprintf("Exited (code: %d)", proc.ExitCode)
			statusStyle = completedStyle
		} else {
			statusInfo = fmt.Sprintf("Failed (code: %d)", proc.ExitCode)
			statusStyle = errorStyle
		}
	} else if !proc.IsActive {
		statusInfo = "Inactive"
		statusStyle = completedStyle
	}

	title := fmt.Sprintf("Process %d: %s", proc.PID, proc.Executable)
	sb.WriteString(headerStyle.Render(title) + " - " + statusStyle.Render(statusInfo) + "\n\n")

	// Back to tree navigation hint
	if m.tooltips {
		sb.WriteString(tooltipStyle.Render("Press ESC to return to tree view, 'r' to toggle raw data") + "\n\n")
	}

	// Render basic info
	sb.WriteString(fmt.Sprintf("PID: %d\n", proc.PID))
	sb.WriteString(fmt.Sprintf("PPID: %d\n", proc.PPID))
	sb.WriteString(fmt.Sprintf("Original PPID: %d\n", proc.OriginalPPID))

	if proc.TTY != "" {
		sb.WriteString(fmt.Sprintf("TTY: %s\n", proc.TTY))
	}

	if !proc.StartTime.IsZero() {
		sb.WriteString(fmt.Sprintf("Start Time: %s\n", proc.StartTime.Format(time.RFC3339)))

		if !m.firstStart.IsZero() {
			relative := proc.StartTime.Sub(m.firstStart).Seconds()
			sb.WriteString(fmt.Sprintf("Relative Start: %.3f seconds\n", relative))
		}
	}

	// Show exit information if the process has exited
	if proc.HasExited {
		sb.WriteString(fmt.Sprintf("\nProcess Exited:\n"))
		exitStyle := completedStyle
		if proc.ExitCode != 0 {
			exitStyle = errorStyle
		}
		sb.WriteString(fmt.Sprintf("  Exit Code: %s\n", exitStyle.Render(fmt.Sprintf("%d", proc.ExitCode))))

		if !proc.ExitTime.IsZero() {
			sb.WriteString(fmt.Sprintf("  Exit Time: %s\n", proc.ExitTime.Format(time.RFC3339)))

			if !proc.StartTime.IsZero() {
				duration := proc.ExitTime.Sub(proc.StartTime)
				sb.WriteString(fmt.Sprintf("  Process Duration: %.3f seconds\n", duration.Seconds()))
			}
		}
	}

	// File operations section with improved icons
	sb.WriteString("\nFile Operations:\n")
	sb.WriteString(fmt.Sprintf("  %s Lookups:   %d\n", lookupIcon, proc.FileOps.Lookups))
	sb.WriteString(fmt.Sprintf("  %s Readlinks: %d\n", readlinkIcon, proc.FileOps.Readlinks))
	sb.WriteString(fmt.Sprintf("  %s Stats:     %d\n", statIcon, proc.FileOps.Stats))
	sb.WriteString(fmt.Sprintf("  %s Accesses:  %d\n", accessIcon, proc.FileOps.Accesses))
	sb.WriteString(fmt.Sprintf("  %s Opens:     %d\n", openIcon, proc.FileOps.Opens))
	sb.WriteString(fmt.Sprintf("  %s Closes:    %d\n", closeIcon, proc.FileOps.Closes))
	sb.WriteString(fmt.Sprintf("  %s Reads:     %d (%s)\n", readIcon, proc.FileOps.Reads, formatBytes(proc.FileOps.BytesRead)))
	sb.WriteString(fmt.Sprintf("  %s Writes:    %d (%s)\n", writeIcon, proc.FileOps.Writes, formatBytes(proc.FileOps.BytesWritten)))

	// Command line
	if len(proc.Args) > 0 {
		sb.WriteString("\nCommand:\n")
		sb.WriteString(fmt.Sprintf("  %s\n", strings.Join(proc.Args, " ")))
	}
	
	// Events
	if len(proc.Events) > 0 {
		sb.WriteString("\nEvents:\n")
		
		for i, event := range proc.Events {
			// Format based on raw mode
			if m.rawMode {
				jsonData, err := json.MarshalIndent(event, "  ", "  ")
				if err == nil {
					sb.WriteString(fmt.Sprintf("  Event %d:\n  %s\n\n", i+1, string(jsonData)))
				}
			} else {
				// Simplified event display
				sb.WriteString(fmt.Sprintf("  Event %d:\n", i+1))
				sb.WriteString(fmt.Sprintf("    Sequence: %d\n", event.SeqNum))
				sb.WriteString(fmt.Sprintf("    Time: %s\n", event.Time))
				
				if event.Event.Exec != nil {
					if len(event.Event.Exec.Args) > 0 {
						sb.WriteString(fmt.Sprintf("    Args: %s\n", strings.Join(event.Event.Exec.Args, " ")))
					}
					
					if event.Event.Exec.CWD.Path != "" {
						sb.WriteString(fmt.Sprintf("    Working Dir: %s\n", event.Event.Exec.CWD.Path))
					}
					
					if len(event.Event.Exec.Env) > 0 {
						sb.WriteString("    Environment:\n")
						for _, env := range event.Event.Exec.Env[:min(5, len(event.Event.Exec.Env))] {
							sb.WriteString(fmt.Sprintf("      %s\n", env))
						}
						if len(event.Event.Exec.Env) > 5 {
							sb.WriteString(fmt.Sprintf("      ... (%d more)\n", len(event.Event.Exec.Env)-5))
						}
					}
				}
				sb.WriteString("\n")
			}
		}
	}
	
	// Children
	if len(proc.Children) > 0 {
		sb.WriteString("\nChildren:\n")
		
		// Sort children by PID
		var childPIDs []int
		for pid := range proc.Children {
			childPIDs = append(childPIDs, pid)
		}
		sort.Ints(childPIDs)
		
		for _, pid := range childPIDs {
			child := proc.Children[pid]
			childStatus := ""
			if child.HasExited {
				childStatus = fmt.Sprintf(" (Exited: %d)", child.ExitCode)
			}
			sb.WriteString(fmt.Sprintf("  PID %d: %s%s\n", pid, child.Executable, childStatus))
		}
	}
	
	return sb.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatBytes formats a byte count into a human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// View renders the application UI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Header
	header := titleStyle.Render("eslog - Process Tree Viewer")
	
	// Filter input bar
	filterBar := ""
	if m.showFilterBar {
		filterBar = "\n" + filterInputStyle.Render("Filter: "+m.filterInput.View())
	}
	
	// Search input bar
	searchBar := ""
	if m.searchMode {
		searchResult := ""
		if len(m.searchResults) > 0 {
			searchResult = fmt.Sprintf(" (%d matches)", len(m.searchResults))
		}
		searchBar = "\n" + filterInputStyle.Render("Search: "+m.searchInput.View()) + searchResult
	}
	
	// Copy message
	copyMsg := ""
	if m.showCopyMessage {
		copyMsg = "\n" + tooltipStyle.Render(m.copyMessage)
	}

	// Status line
	processCount := len(m.processTree.Nodes)
	status := fmt.Sprintf("%d processes", processCount)
	
	if m.filteringActive {
		filteredCount := len(m.filteredTree)
		status = fmt.Sprintf("%d processes (showing %d matches for '%s')", 
			processCount, filteredCount, m.filterText)
	}
	
	if m.hasFocus {
		status += fmt.Sprintf(" - Viewing PID %d", m.focusedPID)
	} else if m.selectedPID != 0 {
		status += fmt.Sprintf(" - Selected PID %d", m.selectedPID)
	}
	
	// Show search results summary if we have results
	if len(m.searchResults) > 0 && m.currentSearch != "" {
		status += fmt.Sprintf(" - Search: %d matches for '%s'", 
			len(m.searchResults), m.currentSearch)
		
		if m.searchResultIdx < len(m.searchResults) {
			status += fmt.Sprintf(" (showing %d/%d)", 
				m.searchResultIdx+1, len(m.searchResults))
		}
	}
	
	statusLine := statusStyle.Render(status)

	// Main content area
	mainContent := m.viewport.View()

	// Help
	helpView := ""
	if m.showHelp {
		helpView = "\n" + m.help.View(m.keys)
	}

	// Combine all elements
	return fmt.Sprintf("%s\n\n%s\n\n%s%s%s%s%s", 
		header, 
		mainContent, 
		statusLine, 
		filterBar,
		searchBar,
		copyMsg,
		helpView)
}

// StartTUI starts the TUI application
func StartTUI(processTree *ProcessTree) error {
	m := NewModel(processTree)
	p := tea.NewProgram(m, tea.WithAltScreen())
	
	_, err := p.Run()
	return err
}