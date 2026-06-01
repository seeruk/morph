package morph

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/seeruk/morph/types"
)

// stableRunHash attempts to build a stable hash for a run of Morph, which is unique to the module,
// working directory, and a stable identifier; in CLI usage, this is just the config file name, as
// none of these things should change between Morph runs.
func stableRunHash(ws *Workspace, ident string) (string, error) {
	relativeToModule, err := filepath.Rel(ws.ModuleDir, ws.WorkingDir)
	if err != nil {
		return "", fmt.Errorf("failed to determine relative working directory: %w", err)
	}

	if relativeToModule == ".." || strings.HasPrefix(relativeToModule, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("expected working directory %q to be inside module %q", ws.WorkingDir, ws.ModuleDir)
	}

	fields := []string{
		"morph-run-id/v1",
		ws.ModulePath,
		relativeToModule,
		ident,
	}

	sum := sha256.Sum256([]byte(strings.Join(fields, "\x00")))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).
		EncodeToString(sum[:])

	return strings.ToLower(encoded[:10]), nil
}

func stableTypePairHash(source, target types.Type) string {
	fields := []string{
		"morph-type-pair/v1",
		types.TypeKey(source),
		types.TypeKey(target),
	}

	sum := sha256.Sum256([]byte(strings.Join(fields, "\x00")))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).
		EncodeToString(sum[:])

	return strings.ToLower(encoded[:10])
}
