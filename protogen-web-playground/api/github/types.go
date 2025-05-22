package github

// PlaygroundConfig represents the playground configuration
type PlaygroundConfig struct {
	Proto struct {
		Files []ProtoFile `json:"files"`
	} `json:"proto"`
	Templates []Template `json:"templates"`
	Options   struct {
		ContinueOnError bool `json:"continueOnError"`
		Verbose         bool `json:"verbose"`
		IncludeImports  bool `json:"includeImports"`
	} `json:"options"`
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

// CreateGistRequest is the request for creating a GitHub Gist
type CreateGistRequest struct {
	Config      *PlaygroundConfig `json:"config"`
	Description string            `json:"description"`
	Public      bool              `json:"public"`
}

// GistResponse is the response from the GitHub Gist API
type GistResponse struct {
	ID          string                     `json:"id"`
	HTMLURL     string                     `json:"html_url"`
	Files       map[string]GistFileContent `json:"files"`
	Description string                     `json:"description"`
}

// GistFileContent represents the content of a file in a Gist
type GistFileContent struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}