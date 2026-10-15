package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Formato de los tracking codes: <capa><archivo>E<nnn>, ej: SLISE001, PRHTTPE003, DVHOOK002
var trackingCodePattern = regexp.MustCompile(`^[A-Z]{2,}[0-9]{3}$`)

// Raiz del proyecto relativa a esta carpeta (go test ejecuta las pruebas dentro de tests/)
const projectRoot = ".."

// TestTrackingCodesAreUnique recorre el codigo del proyecto (sin tests/ ni los _test.go) y falla si un
// tracking code se repite, porque un codigo repetido ya no identifica un unico punto del codigo
func TestTrackingCodesAreUnique(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	positions := map[string][]string{}

	walkErr := filepath.WalkDir(projectRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != projectRoot && (strings.HasPrefix(name, ".") || name == "tests" || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}

		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err == nil && trackingCodePattern.MatchString(value) {
				positions[value] = append(positions[value], fileSet.Position(literal.Pos()).String())
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("error recorriendo el codigo: %v", walkErr)
	}

	if len(positions) == 0 {
		t.Fatal("no se encontro ningun tracking code, revisa trackingCodePattern")
	}

	for code, found := range positions {
		if len(found) > 1 {
			t.Errorf("el tracking code %s se repite en:\n  %s", code, strings.Join(found, "\n  "))
		}
	}
}
