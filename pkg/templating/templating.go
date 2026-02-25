// Package templating provides advanced templating capabilities for cluster templates.
package templating

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

// Context holds the data available to templates.
type Context struct {
	Env        map[string]string
	Cluster    map[string]interface{}
	Plugins    map[string]interface{}
	GitCommit  string
	GitBranch  string
	GitTag     string
	Timestamp  string
}

// NewContext creates a new template context with environment variables.
func NewContext() *Context {
	return &Context{
		Env:       loadEnv(),
		Cluster:   make(map[string]interface{}),
		Plugins:   make(map[string]interface{}),
		Timestamp: fmt.Sprintf("%d", os.Getpid()), // Placeholder, can be replaced with actual timestamp
	}
}

// WithGitInfo adds git information to the context.
func (c *Context) WithGitInfo(commit, branch, tag string) *Context {
	c.GitCommit = commit
	c.GitBranch = branch
	c.GitTag = tag
	return c
}

// WithClusterInfo adds cluster information to the context.
func (c *Context) WithClusterInfo(name, provider string, controlPlanes, workers int) *Context {
	c.Cluster["name"] = name
	c.Cluster["provider"] = provider
	c.Cluster["controlPlanes"] = controlPlanes
	c.Cluster["workers"] = workers
	return c
}

// Engine provides template processing capabilities.
type Engine struct {
	context *Context
	funcs   template.FuncMap
}

// NewEngine creates a new templating engine.
func NewEngine(context *Context) *Engine {
	e := &Engine{
		context: context,
		funcs:   make(template.FuncMap),
	}
	e.registerFunctions()
	return e
}

// ProcessTemplate processes a template string with the engine's context.
func (e *Engine) ProcessTemplate(content string) (string, error) {
	// Check if content contains template expressions
	if !containsTemplateExpressions(content) {
		return content, nil
	}

	tmpl, err := template.New("template").Funcs(e.funcs).Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.context); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ProcessFile reads a file and processes it as a template.
func (e *Engine) ProcessFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return e.ProcessTemplate(string(content))
}

// registerFunctions registers template functions.
func (e *Engine) registerFunctions() {
	// Environment variable access with default
	e.funcs["env"] = func(key string, defaultValue ...string) string {
		if val, ok := e.context.Env[key]; ok {
			return val
		}
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return ""
	}

	// Required environment variable - fails if not set
	e.funcs["required"] = func(key string) (string, error) {
		if val, ok := e.context.Env[key]; ok && val != "" {
			return val, nil
		}
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}

	// Default value
	e.funcs["default"] = func(defaultValue string, value string) string {
		if value != "" {
			return value
		}
		return defaultValue
	}

	// Upper case
	e.funcs["upper"] = strings.ToUpper

	// Lower case
	e.funcs["lower"] = strings.ToLower

	// Title case (simple implementation)
	e.funcs["title"] = func(s string) string {
		if s == "" {
			return ""
		}
		return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	}

	// Replace
	e.funcs["replace"] = strings.ReplaceAll

	// Contains
	e.funcs["contains"] = strings.Contains

	// HasPrefix
	e.funcs["hasPrefix"] = strings.HasPrefix

	// HasSuffix
	e.funcs["hasSuffix"] = strings.HasSuffix

	// Trim
	e.funcs["trim"] = strings.TrimSpace

	// TrimPrefix
	e.funcs["trimPrefix"] = strings.TrimPrefix

	// TrimSuffix
	e.funcs["trimSuffix"] = strings.TrimSuffix

	// Quote - wraps string in quotes
	e.funcs["quote"] = func(s string) string {
		return fmt.Sprintf("%q", s)
	}

	// Indent - indents each line
	e.funcs["indent"] = func(spaces int, s string) string {
		pad := strings.Repeat(" ", spaces)
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = pad + line
			}
		}
		return strings.Join(lines, "\n")
	}

	// Nindent - indents each line and adds newline at start
	e.funcs["nindent"] = func(spaces int, s string) string {
		return "\n" + e.funcs["indent"].(func(int, string) string)(spaces, s)
	}

	// ToYaml - converts to YAML (simple implementation)
	e.funcs["toYaml"] = func(v interface{}) string {
		switch val := v.(type) {
		case string:
			return val
		case int:
			return strconv.Itoa(val)
		case bool:
			return strconv.FormatBool(val)
		default:
			return fmt.Sprintf("%v", v)
		}
	}

	// Cluster info access
	e.funcs["clusterName"] = func() string {
		if name, ok := e.context.Cluster["name"].(string); ok {
			return name
		}
		return ""
	}

	e.funcs["clusterProvider"] = func() string {
		if provider, ok := e.context.Cluster["provider"].(string); ok {
			return provider
		}
		return ""
	}

	// Git info access
	e.funcs["gitCommit"] = func() string {
		return e.context.GitCommit
	}

	e.funcs["gitBranch"] = func() string {
		return e.context.GitBranch
	}

	e.funcs["gitTag"] = func() string {
		return e.context.GitTag
	}
}

// loadEnv loads environment variables into a map.
func loadEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			env[pair[0]] = pair[1]
		}
	}
	return env
}

// containsTemplateExpressions checks if content contains Go template expressions.
func containsTemplateExpressions(content string) bool {
	// Look for {{ ... }} patterns
	re := regexp.MustCompile(`\{\{\s*\.\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\}\}`)
	if re.MatchString(content) {
		return true
	}

	// Look for {{ function ... }} patterns
	re = regexp.MustCompile(`\{\{\s*[a-zA-Z_][a-zA-Z0-9_]*\s+`)
	if re.MatchString(content) {
		return true
	}

	// Look for {{ ... | ... }} pipe patterns
	re = regexp.MustCompile(`\{\{.*\|.*\}\}`)
	return re.MatchString(content)
}

// ProcessTemplateFile reads a template file, processes it, and returns the processed content.
// This is the main entry point for template processing.
func ProcessTemplateFile(path string, envFiles []string) (string, error) {
	// Load additional env files if specified
	for _, envFile := range envFiles {
		if err := loadEnvFile(envFile); err != nil {
			// Non-fatal: env file might not exist
			continue
		}
	}

	// Create context and engine
	ctx := NewContext()

	// Try to get git info
	if commit, err := getGitCommit(); err == nil {
		ctx.GitCommit = commit
	}
	if branch, err := getGitBranch(); err == nil {
		ctx.GitBranch = branch
	}
	if tag, err := getGitTag(); err == nil {
		ctx.GitTag = tag
	}

	engine := NewEngine(ctx)

	return engine.ProcessFile(path)
}

// loadEnvFile loads environment variables from a file.
func loadEnvFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			os.Setenv(key, value)
		}
	}

	return nil
}

// getGitCommit returns the current git commit hash.
func getGitCommit() (string, error) {
	out, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return "", err
	}

	ref := strings.TrimSpace(string(out))
	if strings.HasPrefix(ref, "ref: ") {
		ref = strings.TrimPrefix(ref, "ref: ")
		out, err = os.ReadFile(".git/" + ref)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out))[:7], nil
	}

	return ref[:7], nil
}

// getGitBranch returns the current git branch.
func getGitBranch() (string, error) {
	out, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return "", err
	}

	ref := strings.TrimSpace(string(out))
	if strings.HasPrefix(ref, "ref: refs/heads/") {
		return strings.TrimPrefix(ref, "ref: refs/heads/"), nil
	}

	return "HEAD", nil
}

// getGitTag returns the current git tag (if any).
func getGitTag() (string, error) {
	// Try to read from .git/refs/tags
	entries, err := os.ReadDir(".git/refs/tags")
	if err != nil {
		return "", err
	}

	if len(entries) > 0 {
		// Return the most recent tag (alphabetically last)
		return entries[len(entries)-1].Name(), nil
	}

	return "", fmt.Errorf("no tags found")
}
