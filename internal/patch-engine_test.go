package internal

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewPatchEngine(t *testing.T) {
	pe := NewPatchEngine()
	if pe == nil {
		t.Fatal("NewPatchEngine() returned nil")
	}
	if pe.template == nil {
		t.Error("template should be initialized")
	}
	if pe.data != nil {
		t.Error("data should be nil initially")
	}
}

func TestSetData(t *testing.T) {
	pe := NewPatchEngine()
	testData := map[string]interface{}{
		"name":    "test",
		"version": "1.0",
	}

	pe.SetData(testData)
	if pe.data == nil {
		t.Error("data should be set")
	}
}

func TestPatch(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple text replacement",
			template: "Hello {{.name}}!",
			data:     map[string]string{"name": "World"},
			want:     "Hello World!",
		},
		{
			name:     "multiple replacements",
			template: "{{.greeting}} {{.name}}, version {{.version}}",
			data: map[string]interface{}{
				"greeting": "Hello",
				"name":     "konfig",
				"version":  "1.0.0",
			},
			want: "Hello konfig, version 1.0.0",
		},
		{
			name:     "sprig function usage",
			template: "{{.name | upper}}",
			data:     map[string]string{"name": "konfig"},
			want:     "KONFIG",
		},
		{
			name:     "sprig function - default",
			template: "{{.missing | default \"default-value\"}}",
			data:     map[string]string{},
			want:     "default-value",
		},
		{
			name:     "empty template",
			template: "",
			data:     map[string]string{"name": "test"},
			want:     "",
		},
		{
			name:     "no template variables",
			template: "static content",
			data:     map[string]string{"name": "test"},
			want:     "static content",
		},
		{
			name:     "invalid template syntax",
			template: "{{.name",
			data:     map[string]string{"name": "test"},
			wantErr:  true,
		},
		{
			name:     "missing data field",
			template: "{{.missing}}",
			data:     map[string]string{"name": "test"},
			want:     "<no value>", // Go templates output "<no value>" for missing fields
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pe := NewPatchEngine()
			pe.SetData(tt.data)

			var buf bytes.Buffer
			reader := strings.NewReader(tt.template)

			err := pe.Patch(reader, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("Patch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				got := buf.String()
				if got != tt.want {
					t.Errorf("Patch() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestPatchWithNilData(t *testing.T) {
	pe := NewPatchEngine()
	// Don't set any data (pe.data remains nil)

	template := "static content"
	reader := strings.NewReader(template)
	var buf bytes.Buffer

	err := pe.Patch(reader, &buf)
	if err != nil {
		t.Errorf("Patch() with nil data should work for template without variables, error = %v", err)
	}

	got := buf.String()
	want := "static content"
	if got != want {
		t.Errorf("Patch() = %v, want %v", got, want)
	}
}

func TestPatchReaderError(t *testing.T) {
	pe := NewPatchEngine()
	pe.SetData(map[string]string{"name": "test"})

	// Create a reader that will return an error
	errorReader := &errorReader{}
	var buf bytes.Buffer

	err := pe.Patch(errorReader, &buf)
	if err == nil {
		t.Error("Patch() should return error when reader fails")
	}
}

// errorReader is a helper that always returns an error on Read
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}

func TestPatchComplexTemplate(t *testing.T) {
	pe := NewPatchEngine()

	data := map[string]interface{}{
		"app": map[string]interface{}{
			"name":    "myapp",
			"version": "2.1.0",
			"ports":   []int{8080, 9090},
		},
		"env": "production",
	}
	pe.SetData(data)

	template := `# Configuration for {{.app.name}}
version: {{.app.version}}
environment: {{.env}}
ports:
{{- range .app.ports}}
  - {{.}}
{{- end}}
service_name: {{.app.name}}-{{.env | lower}}`

	reader := strings.NewReader(template)
	var buf bytes.Buffer

	err := pe.Patch(reader, &buf)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}

	expected := `# Configuration for myapp
version: 2.1.0
environment: production
ports:
  - 8080
  - 9090
service_name: myapp-production`

	got := buf.String()
	if got != expected {
		t.Errorf("Patch() = %v, want %v", got, expected)
	}
}
