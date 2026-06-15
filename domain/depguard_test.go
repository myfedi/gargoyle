package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const domainImportPrefix = "github.com/myfedi/gargoyle/domain"

func TestDepGuard(t *testing.T) {
	t.Parallel()

	var violations []string
	fset := token.NewFileSet()

	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if isStdlibImport(importPath) || isDomainImport(importPath) {
				continue
			}
			pos := fset.Position(imp.Pos())
			violations = append(violations, pos.String()+": domain packages may only import stdlib or "+domainImportPrefix+"; found "+importPath)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("depguard failed to inspect domain imports: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("depguard failed:\n%s", strings.Join(violations, "\n"))
	}
}

func isStdlibImport(importPath string) bool {
	return !strings.Contains(importPath, ".")
}

func isDomainImport(importPath string) bool {
	return importPath == domainImportPrefix || strings.HasPrefix(importPath, domainImportPrefix+"/")
}
