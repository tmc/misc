package webkithost

import (
	"fmt"
	"strings"

	"github.com/tmc/apple/appkit"
	"github.com/tmc/apple/uniformtypeidentifiers"
)

func (h *Host) applyPickerSession(name, verb string) error {
	if verb != "show" {
		return nil
	}
	id := strings.Split(name, "/")[2]
	done := make(chan error, 1)
	h.performOnMain(func() {
		done <- h.showPicker(id)
	})
	return <-done
}

func (h *Host) showPicker(id string) error {
	mode := strings.TrimSpace(readAppleFSString(h, "appkit/picker/"+id+"/mode"))
	if mode == "save-file" {
		return h.showSavePanel(id)
	}
	panel := appkit.NewNSOpenPanel()
	panel.SetCanChooseFiles(true)
	panel.SetCanChooseDirectories(false)
	panel.SetAllowsMultipleSelection(true)
	panel.SetResolvesAliases(true)
	configureSavePanel(panel.NSSavePanel, pickerPrompt(h, id), "Open", pickerTypes(h, id))
	switch mode {
	case "open-dir":
		panel.SetCanChooseFiles(false)
		panel.SetCanChooseDirectories(true)
	case "open-any":
		panel.SetCanChooseFiles(true)
		panel.SetCanChooseDirectories(true)
	}
	response := panel.RunModal()
	if response != appkit.NSModalResponses.OK {
		return h.appleFS.WriteFile("appkit/picker/"+id+"/result", []byte("cancelled true\n"))
	}
	var b strings.Builder
	urls := panel.URLs()
	b.WriteString("cancelled false\n")
	b.WriteString(fmt.Sprintf("count %d\n", len(urls)))
	for i, url := range urls {
		b.WriteString(fmt.Sprintf("path%d %s\n", i, url.Path()))
	}
	return h.appleFS.WriteFile("appkit/picker/"+id+"/result", []byte(b.String()))
}

func (h *Host) showSavePanel(id string) error {
	panel := appkit.NewNSSavePanel()
	configureSavePanel(panel, pickerPrompt(h, id), "Save", pickerTypes(h, id))
	panel.SetCanCreateDirectories(true)
	response := panel.RunModal()
	if response != appkit.NSModalResponses.OK {
		return h.appleFS.WriteFile("appkit/picker/"+id+"/result", []byte("cancelled true\n"))
	}
	url := panel.URL()
	return h.appleFS.WriteFile("appkit/picker/"+id+"/result", []byte("cancelled false\ncount 1\npath0 "+url.Path()+"\n"))
}

func configureSavePanel(panel appkit.NSSavePanel, prompt, button string, types []uniformtypeidentifiers.UTType) {
	if prompt != "" {
		panel.SetMessage(prompt)
		panel.SetPrompt(button)
	}
	if len(types) > 0 {
		panel.SetAllowedContentTypes(types)
	}
}

func pickerPrompt(h *Host, id string) string {
	return strings.TrimSpace(readAppleFSString(h, "appkit/picker/"+id+"/prompt"))
}

func pickerTypes(h *Host, id string) []uniformtypeidentifiers.UTType {
	return parsePickerTypes(readAppleFSString(h, "appkit/picker/"+id+"/types"))
}

func parsePickerTypes(text string) []uniformtypeidentifiers.UTType {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == ',' || r == ' ' || r == '\t'
	})
	var out []uniformtypeidentifiers.UTType
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		typ := pickerType(field)
		if typ.ID != 0 {
			out = append(out, typ)
		}
	}
	return out
}

func pickerType(name string) uniformtypeidentifiers.UTType {
	name = strings.TrimPrefix(strings.TrimSpace(name), ".")
	if name == "" {
		return uniformtypeidentifiers.UTType{}
	}
	if strings.Contains(name, "/") {
		return uniformtypeidentifiers.NewTypeWithMIMEType(name)
	}
	if strings.Contains(name, ".") {
		return uniformtypeidentifiers.NewTypeWithIdentifier(name)
	}
	return uniformtypeidentifiers.NewTypeWithFilenameExtension(name)
}
