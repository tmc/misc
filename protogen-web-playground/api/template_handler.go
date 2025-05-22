package api

import (
	"encoding/json"
	"net/http"
	"text/template"

	"github.com/Masterminds/sprig"
	"go.uber.org/zap"
)

// TemplateHandler handles template-related requests
type TemplateHandler struct {
	Logger *zap.Logger
}

// NewTemplateHandler creates a new TemplateHandler
func NewTemplateHandler() *TemplateHandler {
	logger, _ := zap.NewProduction()
	return &TemplateHandler{
		Logger: logger,
	}
}

// HandleGetExamples returns example templates
func (h *TemplateHandler) HandleGetExamples(w http.ResponseWriter, r *http.Request) {
	// Example templates
	examples := []Template{
		{
			Name: "{{.Service.GoName}}_service.go.tmpl",
			Content: `package {{.File.GoPackageName}}

// {{.Service.GoName}}Server is the server API for {{.Service.GoName}} service.
type {{.Service.GoName}}Server interface {
{{- range .Service.Methods}}
	{{.GoName}}(context.Context, *{{.Input.GoIdent.GoName}}) (*{{.Output.GoIdent.GoName}}, error)
{{- end}}
}

// Register{{.Service.GoName}}Server registers the {{.Service.GoName}} server implementation.
func Register{{.Service.GoName}}Server(s *grpc.Server, srv {{.Service.GoName}}Server) {
	s.RegisterService(&{{.File.Services.0.GoName}}_ServiceDesc, srv)
}`,
		},
		{
			Name: "{{.Message.GoIdent.GoName}}_extension.go.tmpl",
			Content: `package {{.File.GoPackageName}}

// Adds Foobar() method to {{.Message.GoIdent}}
func (m *{{.Message.GoIdent.GoName}}) Foobar() {
	// Implementation goes here
}`,
		},
		{
			Name: "{{.Message.GoIdent.GoName}}.http.go.tmpl",
			Content: `package {{.File.GoPackageName}}

import (
	"net/http"
	"encoding/json"
)

// Handler for {{.Message.GoIdent.GoName}}
type {{.Message.GoIdent.GoName}}Handler struct{}

// Get{{.Message.GoIdent.GoName}} handles GET requests
func (h *{{.Message.GoIdent.GoName}}Handler) Get{{.Message.GoIdent.GoName}}(w http.ResponseWriter, r *http.Request) {
	response := &{{.Message.GoIdent.GoName}}{}
	json.NewEncoder(w).Encode(response)
}`,
		},
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(examples); err != nil {
		h.Logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleValidate validates a template
func (h *TemplateHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req TemplateValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate template
	_, err := template.New("template").
		Funcs(sprig.TxtFuncMap()).
		Funcs(template.FuncMap{
			// Add stub functions to make validation work
			"methodExtension":  func(a, b interface{}) interface{} { return nil },
			"messageExtension": func(a, b interface{}) interface{} { return nil },
			"fieldExtension":   func(a, b interface{}) interface{} { return nil },
			"fieldByName":      func(a, b interface{}) interface{} { return nil },
		}).
		Parse(req.Template)

	// Create response
	resp := TemplateValidateResponse{
		Valid: err == nil,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.Logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}