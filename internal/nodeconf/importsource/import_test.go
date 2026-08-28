package importsource

import "testing"

func TestInheritsModulePath(t *testing.T) {
	if !(&ImportString{}).InheritsModulePath() {
		t.Fatal("import.string should inherit module_path from the parent scope")
	}

	notInheriting := map[string]ImportSource{
		"import.file": &ImportFile{},
		"import.git":  &ImportGit{},
		"import.http": &ImportHTTP{},
	}
	for name, src := range notInheriting {
		if src.InheritsModulePath() {
			t.Fatalf("%s should define its own module_path, not inherit it", name)
		}
	}
}
