package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ToolFunc is the handler for a tool.
type ToolFunc func(args map[string]interface{}) (string, error)

// ToolDef defines a tool: its schema and handler.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Handler     ToolFunc
}

// Registry holds all available tools.
type Registry struct {
	tools map[string]*ToolDef
}

func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]*ToolDef)}
	r.register(ToolDef{
		Name:        "read_file",
		Description: "Read the contents of a file.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read.",
				},
			},
			"required": []interface{}{"path"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	})
	r.register(ToolDef{
		Name:        "write_file",
		Description: "Write or overwrite a file with the given content.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to write.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write to the file.",
				},
			},
			"required": []interface{}{"path", "content"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return "", err
			}
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", err
			}
			return fmt.Sprintf("Wrote %d bytes to %s", len(content), path), nil
		},
	})
	r.register(ToolDef{
		Name:        "edit_file",
		Description: "Replace a specific string in a file with another string.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to edit.",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "The exact text to replace.",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "The text to replace it with.",
				},
			},
			"required": []interface{}{"path", "old_string", "new_string"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			oldStr, _ := args["old_string"].(string)
			newStr, _ := args["new_string"].(string)

			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			content := string(data)
			if !strings.Contains(content, oldStr) {
				return "", fmt.Errorf("old_string not found in %s", path)
			}
			content = strings.Replace(content, oldStr, newStr, 1)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", err
			}
			return fmt.Sprintf("Edited %s", path), nil
		},
	})
	r.register(ToolDef{
		Name:        "ls",
		Description: "List files and directories in a path.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path to list. Defaults to current directory.",
				},
			},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return "", err
			}
			var lines []string
			for _, e := range entries {
				info, _ := e.Info()
				prefix := "-"
				if e.IsDir() {
					prefix = "d"
				}
				lines = append(lines, fmt.Sprintf("%s %10d %s", prefix, info.Size(), e.Name()))
			}
			sort.Strings(lines)
			return strings.Join(lines, "\n"), nil
		},
	})
	r.register(ToolDef{
		Name:        "grep",
		Description: "Search for a pattern in file contents.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Regex pattern to search for.",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory or file to search in.",
				},
			},
			"required": []interface{}{"pattern"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			pattern, _ := args["pattern"].(string)
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return "", err
			}

			var matches []string
			filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return nil
				}
				lines := strings.Split(string(data), "\n")
				for i, line := range lines {
					if re.MatchString(line) {
						matches = append(matches, fmt.Sprintf("%s:%d: %s", p, i+1, line))
					}
				}
				return nil
			})

			if len(matches) == 0 {
				return "No matches found.", nil
			}
			return strings.Join(matches, "\n"), nil
		},
	})
	r.register(ToolDef{
		Name:        "glob",
		Description: "Find files matching a glob pattern. Supports ** for recursive matching.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Glob pattern (e.g. '**/*.go').",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Base directory for the search.",
				},
			},
			"required": []interface{}{"pattern"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			pattern, _ := args["pattern"].(string)
			path, _ := args["path"].(string)
			if path == "" {
				path = "."
			}

			// Support ** for recursive matching
			var matches []string
			if strings.Contains(pattern, "**") {
				filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(path, p)
					if err != nil {
						return nil
					}
					ok, err := filepath.Match(pattern, rel)
					if err != nil {
						return nil
					}
					if ok {
						matches = append(matches, p)
					}
					return nil
				})
			} else {
				searchPath := filepath.Join(path, pattern)
				var err error
				matches, err = filepath.Glob(searchPath)
				if err != nil {
					return "", err
				}
			}

			if len(matches) == 0 {
				return "No matches found.", nil
			}
			sort.Strings(matches)
			return strings.Join(matches, "\n"), nil
		},
	})
	r.register(ToolDef{
		Name:        "run_command",
		Description: "Run a shell command and return its output.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "Shell command to execute.",
				},
				"timeout": map[string]interface{}{
					"type":        "number",
					"description": "Timeout in seconds. Default 30.",
				},
			},
			"required": []interface{}{"command"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			command, _ := args["command"].(string)
			timeout := 30
			if t, ok := args["timeout"].(float64); ok {
				timeout = int(t)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out) + "\nError: " + err.Error(), nil
			}
			return string(out), nil
		},
	})
	r.register(ToolDef{
		Name:        "todo_write",
		Description: "Manage a todo list.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"todos": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{"type": "string"},
							"status":  map[string]interface{}{"type": "string"},
						},
					},
				},
			},
			"required": []interface{}{"todos"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			todosRaw, _ := args["todos"].([]interface{})
			var lines []string
			for _, t := range todosRaw {
				tm, ok := t.(map[string]interface{})
				if !ok {
					continue
				}
				content, _ := tm["content"].(string)
				status, _ := tm["status"].(string)
				lines = append(lines, fmt.Sprintf("  [%s] %s", status, content))
			}
			return "Todo list:\n" + strings.Join(lines, "\n"), nil
		},
	})
	return r
}

func (r *Registry) register(t ToolDef) {
	r.tools[t.Name] = &t
}

// List returns all tool definitions as OpenAI-compatible tool specs.
func (r *Registry) List() []Tool {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := []Tool{}
	for _, name := range names {
		t := r.tools[name]
		tools = append(tools, Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return tools
}

// Call executes a tool by name with the given arguments.
func (r *Registry) Call(name, arguments string) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	var args map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments for %s: %w", name, err)
		}
	}

	return t.Handler(args)
}
