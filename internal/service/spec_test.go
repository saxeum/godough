package service

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/saxeum/godough/internal/domain/snippet"
)

type constraint struct {
	MaxLength *int `yaml:"maxLength"`
	MinLength *int `yaml:"minLength"`
	Minimum   *int `yaml:"minimum"`
	Maximum   *int `yaml:"maximum"`
}

type openAPISpec struct {
	Components struct {
		Schemas map[string]struct {
			Properties map[string]constraint `yaml:"properties"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

func loadCreateSnippetRequest(t *testing.T) map[string]constraint {
	t.Helper()

	path := filepath.Join("..", "..", "spec", "openapi.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	schema, ok := spec.Components.Schemas["CreateSnippetRequest"]
	if !ok {
		t.Fatal("the spec has no CreateSnippetRequest schema")
	}
	return schema.Properties
}

func TestSpecConstraintsMatchTheService(t *testing.T) {
	props := loadCreateSnippetRequest(t)

	tests := []struct {
		field  string
		rule   string
		want   int
		actual *int
	}{
		{field: "title", rule: "maxLength", want: snippet.TitleMaxRunes, actual: props["title"].MaxLength},
		{field: "content", rule: "minLength", want: 1, actual: props["content"].MinLength},
		{field: "expiresInDays", rule: "minimum", want: snippet.ExpiryMinDays, actual: props["expiresInDays"].Minimum},
		{field: "expiresInDays", rule: "maximum", want: snippet.ExpiryMaxDays, actual: props["expiresInDays"].Maximum},
	}

	for _, tt := range tests {
		t.Run(tt.field+"."+tt.rule, func(t *testing.T) {
			if tt.actual == nil {
				t.Fatalf("the spec declares no %s on %s, but the service enforces %d", tt.rule, tt.field, tt.want)
			}
			if *tt.actual != tt.want {
				t.Errorf("spec %s.%s = %d, service enforces %d", tt.field, tt.rule, *tt.actual, tt.want)
			}
		})
	}
}
