package types

type ReplaceEntry struct {
	Comment     string   `yaml:"comment"`
	Dependency  string   `yaml:"dependency"`
	Replacement string   `yaml:"replacement"`
	Scope       []string `yaml:"scope"`
}

type Module struct {
	Name       string `yaml:"name"`
	Path       string `yaml:"path"`
	FileType   string `yaml:"file_type"` // "mod" or "ocb"
	OutputFile string `yaml:"output_file"`
}

type ProjectReplaces struct {
	Modules  []Module       `yaml:"modules"`
	Replaces []ReplaceEntry `yaml:"replaces"`
}
