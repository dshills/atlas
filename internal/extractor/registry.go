package extractor

import "fmt"

// Registry maps languages to extractors.
type Registry struct {
	extractors map[string]Extractor
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{extractors: make(map[string]Extractor)}
}

// Register adds an extractor to the registry.
func (r *Registry) Register(e Extractor) {
	r.extractors[e.Language()] = e
}

// ForPath returns the extractor that supports the given file path.
func (r *Registry) ForPath(path string) (Extractor, error) {
	for _, e := range r.extractors {
		if e.Supports(path) {
			return e, nil
		}
	}
	return nil, fmt.Errorf("no extractor for %s", path)
}

// ForLanguage returns the extractor for a specific language.
func (r *Registry) ForLanguage(lang string) (Extractor, bool) {
	e, ok := r.extractors[lang]
	return e, ok
}

// Languages returns all registered languages.
func (r *Registry) Languages() []string {
	langs := make([]string, 0, len(r.extractors))
	for lang := range r.extractors {
		langs = append(langs, lang)
	}
	return langs
}
