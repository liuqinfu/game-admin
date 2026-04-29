package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServicePackagesOnlyUseApprovedCrossServiceImports(t *testing.T) {
	t.Parallel()

	const moduleServicePrefix = "game-admin/backend/internal/services/"

	allowed := map[string][]string{}

	root := filepath.Join("..")
	servicesRoot := filepath.Join(root, "services")
	fileSet := token.NewFileSet()
	var violations []string

	err := filepath.WalkDir(servicesRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() {
			if entry.Name() == "shared" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		require.NoError(t, err)
		relativePath = filepath.ToSlash(relativePath)

		parsedFile, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		require.NoError(t, err)

		serviceName := strings.Split(strings.TrimPrefix(relativePath, "services/"), "/")[0]
		allowedImports := allowed[relativePath]
		for _, importSpec := range parsedFile.Imports {
			importPath := strings.Trim(importSpec.Path.Value, `"`)
			if !strings.HasPrefix(importPath, moduleServicePrefix) {
				continue
			}

			importedService := strings.Split(strings.TrimPrefix(importPath, moduleServicePrefix), "/")[0]
			if importedService == "shared" || importedService == serviceName {
				continue
			}
			if slices.Contains(allowedImports, importPath) {
				continue
			}
			violations = append(violations, relativePath+" -> "+importPath)
		}
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, violations, "unexpected cross-service imports detected")
}
