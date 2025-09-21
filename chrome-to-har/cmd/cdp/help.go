package main

import (
	"fmt"
	"sort"
	"strings"
)

// HelpSystem provides context-aware help for the CDP tool
type HelpSystem struct {
	registry *CommandRegistry
}

// NewHelpSystem creates a new help system
func NewHelpSystem(registry *CommandRegistry) *HelpSystem {
	return &HelpSystem{
		registry: registry,
	}
}

// ShowHelp displays general help or help for a specific command
func (h *HelpSystem) ShowHelp(args []string) {
	if len(args) == 0 {
		h.showGeneralHelp()
	} else {
		h.showCommandHelp(args[0])
	}
}

// showGeneralHelp displays the main help screen
func (h *HelpSystem) showGeneralHelp() {
	fmt.Println("\n╭─────────────────────────────────────────────────╮")
	fmt.Println("│         CDP - Chrome DevTools Protocol CLI      │")
	fmt.Println("╰─────────────────────────────────────────────────╯")
	fmt.Println("\nUsage: cdp [options] [command] [arguments]")
	fmt.Println("\nCommands by Category:")
	fmt.Println("─────────────────────")

	// Get categories and sort them
	categories := h.registry.ListCategories()
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	// Display commands grouped by category
	for _, cat := range categories {
		fmt.Printf("\n%s Commands:\n", cat.Name)

		// Sort commands within category
		sort.Slice(cat.Commands, func(i, j int) bool {
			return cat.Commands[i].Name < cat.Commands[j].Name
		})

		for _, cmd := range cat.Commands {
			aliasStr := ""
			if len(cmd.Aliases) > 0 {
				aliasStr = fmt.Sprintf(" [%s]", strings.Join(cmd.Aliases, ", "))
			}
			fmt.Printf("  %-20s %s%s\n", cmd.Name, cmd.Description, aliasStr)
		}
	}

	fmt.Println("\nSpecial Commands:")
	fmt.Println("─────────────────")
	fmt.Println("  help [command]       Show help for a specific command")
	fmt.Println("  list                 List all available commands")
	fmt.Println("  search <term>        Search for commands")
	fmt.Println("  exit/quit            Exit the program")

	fmt.Println("\nExamples:")
	fmt.Println("─────────")
	fmt.Println("  cdp navigate https://example.com")
	fmt.Println("  cdp click #submit")
	fmt.Println("  cdp screenshot page.png")
	fmt.Println("  cdp eval document.title")

	fmt.Println("\nTips:")
	fmt.Println("─────")
	fmt.Println("  • Use 'help <command>' for detailed command information")
	fmt.Println("  • Most commands have shorter aliases for convenience")
	fmt.Println("  • Commands support tab completion (when available)")
	fmt.Println("  • Use -v or --verbose for detailed output")
	fmt.Println()
}

// showCommandHelp displays detailed help for a specific command
func (h *HelpSystem) showCommandHelp(cmdName string) {
	cmd, found := h.registry.GetCommand(cmdName)
	if !found {
		fmt.Printf("Command '%s' not found. Use 'help' to see all commands.\n", cmdName)

		// Suggest similar commands
		suggestions := h.findSimilarCommands(cmdName)
		if len(suggestions) > 0 {
			fmt.Println("\nDid you mean:")
			for _, s := range suggestions {
				fmt.Printf("  • %s\n", s)
			}
		}
		return
	}

	// Display detailed command help
	fmt.Printf("\n╭─────────────────────────────────────────────────╮\n")
	fmt.Printf("│  Command: %-37s │\n", cmd.Name)
	fmt.Printf("╰─────────────────────────────────────────────────╯\n")

	fmt.Printf("\nDescription:\n  %s\n", cmd.Description)

	fmt.Printf("\nUsage:\n  %s\n", cmd.Usage)

	if len(cmd.Aliases) > 0 {
		fmt.Printf("\nAliases:\n  %s\n", strings.Join(cmd.Aliases, ", "))
	}

	if len(cmd.Examples) > 0 {
		fmt.Println("\nExamples:")
		for _, ex := range cmd.Examples {
			fmt.Printf("  • %s\n", ex)
		}
	}

	fmt.Printf("\nCategory: %s\n", cmd.Category)

	// Show related commands
	related := h.findRelatedCommands(cmd)
	if len(related) > 0 {
		fmt.Println("\nRelated Commands:")
		for _, r := range related {
			if r.Name != cmd.Name {
				fmt.Printf("  • %-15s %s\n", r.Name, r.Description)
			}
		}
	}

	fmt.Println()
}

// ListCommands displays all available commands
func (h *HelpSystem) ListCommands() {
	fmt.Println("\nAll Available Commands:")
	fmt.Println("───────────────────────")

	// Get all commands
	var allCommands []*Command
	for _, cmd := range h.registry.commands {
		allCommands = append(allCommands, cmd)
	}

	// Sort alphabetically
	sort.Slice(allCommands, func(i, j int) bool {
		return allCommands[i].Name < allCommands[j].Name
	})

	// Display in columns
	for i := 0; i < len(allCommands); i += 3 {
		for j := 0; j < 3 && i+j < len(allCommands); j++ {
			fmt.Printf("  %-25s", allCommands[i+j].Name)
		}
		fmt.Println()
	}

	fmt.Printf("\nTotal: %d commands\n", len(allCommands))
	fmt.Println()
}

// SearchCommands searches for commands matching a term
func (h *HelpSystem) SearchCommands(term string) {
	term = strings.ToLower(term)
	var matches []*Command

	// Search in command names, descriptions, and aliases
	for _, cmd := range h.registry.commands {
		if strings.Contains(strings.ToLower(cmd.Name), term) ||
		   strings.Contains(strings.ToLower(cmd.Description), term) {
			matches = append(matches, cmd)
			continue
		}

		// Check aliases
		for _, alias := range cmd.Aliases {
			if strings.Contains(strings.ToLower(alias), term) {
				matches = append(matches, cmd)
				break
			}
		}
	}

	if len(matches) == 0 {
		fmt.Printf("No commands found matching '%s'\n", term)
		return
	}

	fmt.Printf("\nCommands matching '%s':\n", term)
	fmt.Println("───────────────────────")

	// Sort by relevance (name matches first)
	sort.Slice(matches, func(i, j int) bool {
		iNameMatch := strings.Contains(strings.ToLower(matches[i].Name), term)
		jNameMatch := strings.Contains(strings.ToLower(matches[j].Name), term)

		if iNameMatch && !jNameMatch {
			return true
		}
		if !iNameMatch && jNameMatch {
			return false
		}

		return matches[i].Name < matches[j].Name
	})

	for _, cmd := range matches {
		fmt.Printf("\n%-15s %s\n", cmd.Name, cmd.Description)
		if cmd.Usage != "" {
			fmt.Printf("  Usage: %s\n", cmd.Usage)
		}
		if len(cmd.Aliases) > 0 {
			fmt.Printf("  Aliases: %s\n", strings.Join(cmd.Aliases, ", "))
		}
	}

	fmt.Println()
}

// findSimilarCommands finds commands with similar names
func (h *HelpSystem) findSimilarCommands(name string) []string {
	var similar []string
	name = strings.ToLower(name)

	for _, cmd := range h.registry.commands {
		cmdName := strings.ToLower(cmd.Name)

		// Check for prefix match
		if strings.HasPrefix(cmdName, name) || strings.HasPrefix(name, cmdName) {
			similar = append(similar, cmd.Name)
			continue
		}

		// Check for substring match
		if strings.Contains(cmdName, name) || strings.Contains(name, cmdName) {
			similar = append(similar, cmd.Name)
			continue
		}

		// Check aliases
		for _, alias := range cmd.Aliases {
			if strings.Contains(strings.ToLower(alias), name) {
				similar = append(similar, cmd.Name)
				break
			}
		}
	}

	// Limit to top 5 suggestions
	if len(similar) > 5 {
		similar = similar[:5]
	}

	return similar
}

// findRelatedCommands finds commands in the same category
func (h *HelpSystem) findRelatedCommands(cmd *Command) []*Command {
	var related []*Command

	if cat, ok := h.registry.categories[cmd.Category]; ok {
		for _, c := range cat.Commands {
			if c.Name != cmd.Name {
				related = append(related, c)
			}
		}
	}

	// Limit to 5 related commands
	if len(related) > 5 {
		related = related[:5]
	}

	return related
}

// ShowQuickReference displays a quick reference card
func (h *HelpSystem) ShowQuickReference() {
	fmt.Println("\n╭───────────────────────────────────────────────────────────╮")
	fmt.Println("│                   CDP Quick Reference                      │")
	fmt.Println("├───────────────────────────────────────────────────────────┤")
	fmt.Println("│ Navigation          │ DOM Manipulation                     │")
	fmt.Println("├────────────────────┼──────────────────────────────────────┤")
	fmt.Println("│ goto <url>         │ click <selector>                     │")
	fmt.Println("│ reload             │ type <selector> <text>               │")
	fmt.Println("│ back / forward     │ clear <selector>                     │")
	fmt.Println("│ stop               │ submit <form>                        │")
	fmt.Println("├────────────────────┼──────────────────────────────────────┤")
	fmt.Println("│ Page Info          │ Storage                              │")
	fmt.Println("├────────────────────┼──────────────────────────────────────┤")
	fmt.Println("│ title              │ localStorage / ls                    │")
	fmt.Println("│ url                │ setLocal <key> <value>               │")
	fmt.Println("│ html [selector]    │ getLocal <key>                       │")
	fmt.Println("│ screenshot [file]  │ clearLocal                           │")
	fmt.Println("├────────────────────┼──────────────────────────────────────┤")
	fmt.Println("│ Network            │ Emulation                            │")
	fmt.Println("├────────────────────┼──────────────────────────────────────┤")
	fmt.Println("│ cookies            │ mobile / desktop                     │")
	fmt.Println("│ setcookie <n> <v>  │ viewport <width> <height>            │")
	fmt.Println("│ deletecookie <n>   │ offline / online                     │")
	fmt.Println("│ clearcookies       │                                      │")
	fmt.Println("├────────────────────┴──────────────────────────────────────┤")
	fmt.Println("│ JavaScript: eval <expression> | js <code> | exec <script>  │")
	fmt.Println("╰───────────────────────────────────────────────────────────╯")
	fmt.Println()
}

// GetCompletions returns command completions for a partial input
func (h *HelpSystem) GetCompletions(partial string) []string {
	var completions []string
	partial = strings.ToLower(partial)

	// Check command names
	for name := range h.registry.commands {
		if strings.HasPrefix(strings.ToLower(name), partial) {
			completions = append(completions, name)
		}
	}

	// Check aliases
	for alias, cmdName := range h.registry.aliases {
		if strings.HasPrefix(strings.ToLower(alias), partial) {
			completions = append(completions, alias)
			// Also add the full command name as a suggestion
			if !contains(completions, cmdName) {
				completions = append(completions, cmdName)
			}
		}
	}

	sort.Strings(completions)
	return completions
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}