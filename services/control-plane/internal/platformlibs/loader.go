package platformlibs

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

//go:embed all:files
var embedded embed.FS

type meta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PlatformLib is a built-in library shipped with the control plane binary.
type PlatformLib struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Language    string    `json:"language"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Code and Docs are populated by Load but omitted in JSON list responses.
	Code string `json:"code,omitempty"`
	Docs string `json:"docs,omitempty"`
}

// ImportPath returns the language-specific import path for a platform lib.
//
//	bun:    @velane/{slug}
//	python: velane.{slug_with_underscores}
func ImportPath(language, slug string) string {
	switch language {
	case "python":
		return "velane." + strings.ReplaceAll(slug, "-", "_")
	default:
		return "@velane/" + slug
	}
}

// Load reads all platform libraries from the embedded file system and returns
// them sorted by language then slug. Each library's Code field is populated.
//
// Expected layout inside the embedded files/ dir:
//
//	files/{language}/{slug}/meta.json
//	files/{language}/{slug}/index.ts   (bun)
//	files/{language}/{slug}/module.py  (python)
func Load() ([]PlatformLib, error) {
	var libs []PlatformLib

	langEntries, err := fs.ReadDir(embedded, "files")
	if err != nil {
		return nil, fmt.Errorf("platformlibs: read root: %w", err)
	}

	for _, langEntry := range langEntries {
		if !langEntry.IsDir() {
			continue
		}
		language := langEntry.Name()

		slugEntries, err := fs.ReadDir(embedded, "files/"+language)
		if err != nil {
			return nil, fmt.Errorf("platformlibs: read language %s: %w", language, err)
		}

		for _, slugEntry := range slugEntries {
			if !slugEntry.IsDir() {
				continue
			}
			slug := slugEntry.Name()
			dir := "files/" + language + "/" + slug

			metaBytes, err := embedded.ReadFile(dir + "/meta.json")
			if err != nil {
				return nil, fmt.Errorf("platformlibs: %s/%s missing meta.json: %w", language, slug, err)
			}
			var m meta
			if err := json.Unmarshal(metaBytes, &m); err != nil {
				return nil, fmt.Errorf("platformlibs: %s/%s bad meta.json: %w", language, slug, err)
			}

			codeFile := codeFileName(language)
			codeBytes, err := embedded.ReadFile(dir + "/" + codeFile)
			if err != nil {
				return nil, fmt.Errorf("platformlibs: %s/%s missing %s: %w", language, slug, codeFile, err)
			}

			var docs string
			if docsBytes, err := embedded.ReadFile(dir + "/README.md"); err == nil {
				docs = string(docsBytes)
			}

			libs = append(libs, PlatformLib{
				ID:          language + "-" + slug,
				Slug:        slug,
				Language:    language,
				Name:        m.Name,
				Description: m.Description,
				Code:        string(codeBytes),
				Docs:        docs,
			})
		}
	}
	return libs, nil
}

func codeFileName(language string) string {
	if language == "python" {
		return "module.py"
	}
	return "index.ts"
}
