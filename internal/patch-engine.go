package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
)

type PatchEngine struct {
	template     *template.Template
	data         interface{}
	allowedPaths map[string]struct{}
}

func NewPatchEngine() *PatchEngine {
	pe := &PatchEngine{
		allowedPaths: make(map[string]struct{}),
	}

	funcMap := make(template.FuncMap, 1)
	funcMap["readFile"] = func(filePath string) (string, error) {
		if !filepath.IsAbs(filePath) {
			return "", fmt.Errorf("failed to read file %q: path is not absolute", filePath)
		}
		fileName := filepath.Base(filePath)
		if len(fileName) > 0 && fileName[0] == '.' {
			return "", fmt.Errorf("failed to read file %q: reading from hidden files is prohibited", filePath)
		}
		if _, exists := pe.allowedPaths[filepath.Dir(filePath)]; !exists {
			return "", fmt.Errorf("failed to read file %q: path is not allowed", filePath)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %q: %w", filePath, err)
		}
		return string(content), nil
	}

	pe.template = template.New("patch").Funcs(sprig.TxtFuncMap()).Funcs(funcMap)
	return pe
}

func (pe *PatchEngine) AllowPath(path string) {
	pe.allowedPaths[path] = struct{}{}
}

func (pe *PatchEngine) DenyPath(path string) {
	delete(pe.allowedPaths, path)
}

func (pe *PatchEngine) SetData(data interface{}) {
	pe.data = data
}

func (pe *PatchEngine) Patch(in io.Reader, out io.Writer) error {
	// Read the entire template content
	templateBytes, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Parse the template
	tmpl, err := pe.template.Parse(string(templateBytes))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute the template with the provided data
	err = tmpl.Execute(out, pe.data)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
