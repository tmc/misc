package ghascript

// Workflow represents a parsed GitHub Actions workflow
type Workflow struct {
	Name string                 `yaml:"name"`
	On   interface{}            `yaml:"on"`
	Jobs map[string]Job         `yaml:"jobs"`
	Env  map[string]string      `yaml:"env"`
}

// Job represents a workflow job
type Job struct {
	Name        string                 `yaml:"name"`
	RunsOn      interface{}            `yaml:"runs-on"`
	Strategy    *Strategy              `yaml:"strategy"`
	Steps       []Step                 `yaml:"steps"`
	Env         map[string]string      `yaml:"env"`
	If          string                 `yaml:"if"`
	Needs       interface{}            `yaml:"needs"`
	Services    map[string]Service     `yaml:"services"`
	Container   interface{}            `yaml:"container"`
	TimeoutMinutes int                 `yaml:"timeout-minutes"`
}

// Strategy represents job strategy including matrix
type Strategy struct {
	Matrix       map[string]interface{} `yaml:"matrix"`
	FailFast     *bool                  `yaml:"fail-fast"`
	MaxParallel  int                    `yaml:"max-parallel"`
}

// Step represents a workflow step
type Step struct {
	Name             string                 `yaml:"name"`
	Id               string                 `yaml:"id"`
	Uses             string                 `yaml:"uses"`
	Run              string                 `yaml:"run"`
	With             map[string]interface{} `yaml:"with"`
	Env              map[string]string      `yaml:"env"`
	If               string                 `yaml:"if"`
	Shell            string                 `yaml:"shell"`
	WorkingDirectory string                 `yaml:"working-directory"`
	ContinueOnError  bool                   `yaml:"continue-on-error"`
	TimeoutMinutes   int                    `yaml:"timeout-minutes"`
}

// Service represents a service container
type Service struct {
	Image       string            `yaml:"image"`
	Env         map[string]string `yaml:"env"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Options     string            `yaml:"options"`
	Credentials map[string]string `yaml:"credentials"`
}

// Event represents different GitHub events that can trigger workflows
type Event struct {
	Push         *PushEvent         `yaml:"push"`
	PullRequest  *PullRequestEvent  `yaml:"pull_request"`
	WorkflowDispatch *WorkflowDispatchEvent `yaml:"workflow_dispatch"`
	Schedule     []ScheduleEvent    `yaml:"schedule"`
	Release      *ReleaseEvent      `yaml:"release"`
}

// PushEvent represents a push event configuration
type PushEvent struct {
	Branches     []string `yaml:"branches"`
	BranchesIgnore []string `yaml:"branches-ignore"`
	Tags         []string `yaml:"tags"`
	TagsIgnore   []string `yaml:"tags-ignore"`
	Paths        []string `yaml:"paths"`
	PathsIgnore  []string `yaml:"paths-ignore"`
}

// PullRequestEvent represents a pull request event configuration
type PullRequestEvent struct {
	Types        []string `yaml:"types"`
	Branches     []string `yaml:"branches"`
	BranchesIgnore []string `yaml:"branches-ignore"`
	Paths        []string `yaml:"paths"`
	PathsIgnore  []string `yaml:"paths-ignore"`
}

// WorkflowDispatchEvent represents a manual workflow trigger
type WorkflowDispatchEvent struct {
	Inputs map[string]WorkflowInput `yaml:"inputs"`
}

// WorkflowInput represents an input for workflow_dispatch
type WorkflowInput struct {
	Description string      `yaml:"description"`
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default"`
	Type        string      `yaml:"type"`
	Options     []string    `yaml:"options"`
}

// ScheduleEvent represents a scheduled event
type ScheduleEvent struct {
	Cron string `yaml:"cron"`
}

// ReleaseEvent represents a release event
type ReleaseEvent struct {
	Types []string `yaml:"types"`
}