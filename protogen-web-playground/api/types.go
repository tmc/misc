package api

// GenerateRequest is the request for generating code from proto files and templates
type GenerateRequest struct {
	Proto struct {
		Files []ProtoFile `json:"files"`
	} `json:"proto"`
	Templates []Template `json:"templates"`
	Options   Options    `json:"options"`
}

// ProtoFile represents a Protocol Buffer file
type ProtoFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// Template represents a Go template file
type Template struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// Options represents generation options
type Options struct {
	ContinueOnError bool `json:"continueOnError"`
	Verbose         bool `json:"verbose"`
	IncludeImports  bool `json:"includeImports"`
}

// GenerateResponse is the response from the generation API
type GenerateResponse struct {
	Success bool          `json:"success"`
	Files   []OutputFile  `json:"files"`
	Logs    []LogMessage  `json:"logs"`
	Errors  []string      `json:"errors"`
}

// OutputFile represents a generated output file
type OutputFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// LogMessage represents a log message from the generation process
type LogMessage struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// TemplateValidateRequest is the request for validating a template
type TemplateValidateRequest struct {
	Template string `json:"template"`
}

// TemplateValidateResponse is the response from the template validation API
type TemplateValidateResponse struct {
	Valid  bool   `json:"valid"`
	Error  string `json:"error,omitempty"`
}