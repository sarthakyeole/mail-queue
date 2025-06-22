package templates

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"path/filepath"
	"strings"
)

//go:embed html/*.html
var templateFS embed.FS

type Manager struct {
	templates map[string]*template.Template
}

func New() (*Manager, error) {
	manager := &Manager{
		templates: make(map[string]*template.Template),
	}

	if _, err := fs.Stat(templateFS, "html"); err != nil {
		return nil, fmt.Errorf("html template directory not found: %w", err)
	}

	err := fs.WalkDir(templateFS, "html", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking template directory: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".html") {
			return nil
		}

		name := strings.TrimSuffix(filepath.Base(path), ".html")

		content, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", path, err)
		}

		tmpl, err := template.New(name).
			Funcs(template.FuncMap{
				"safeHTML": func(s string) template.HTML {
					return template.HTML(s)
				},
				"safeURL": func(s string) template.URL {
					return template.URL(s)
				},
				"escapeHTML": func(s string) string {
					return template.HTMLEscapeString(s)
				},
			}).
			Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", path, err)
		}

		manager.templates[name] = tmpl
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("template loading failed: %w", err)
	}

	if len(manager.templates) == 0 {
		return nil, fmt.Errorf("no templates found in html directory")
	}

	return manager, nil
}

func (m *Manager) Render(name string, data map[string]interface{}) (string, error) {
	tmpl, ok := m.templates[name]
	if !ok {
		availabletemplates := make([]string, 0, len(m.templates))
		for t := range m.templates {
			availabletemplates = append(availabletemplates, t)
		}
		return "", fmt.Errorf("template '%s' not found. Available templates: %v",
			name, availabletemplates)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template '%s': %w", name, err)
	}

	return buf.String(), nil
}

func (m *Manager) RenderWithSafeURLs(name string, data map[string]interface{}) (string, error) {
	safeData := make(map[string]interface{})
	for key, value := range data {
		safeData[key] = value
	}

	urlFields := []string{"resetUrl", "verifyUrl", "loginUrl", "signupUrl"}
	for _, field := range urlFields {
		if rawURL, ok := data[field].(string); ok && rawURL != "" {
			parsedURL, err := url.Parse(rawURL)
			if err != nil {
				return "", fmt.Errorf("invalid %s: %w", field, err)
			}

			safeData[field] = template.URL(parsedURL.String())
		}
	}

	return m.Render(name, safeData)
}

func (m *Manager) ListAvailabletemplates() []string {
	templates := make([]string, 0, len(m.templates))
	for name := range m.templates {
		templates = append(templates, name)
	}
	return templates
}
