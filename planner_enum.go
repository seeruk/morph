package morph

import (
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
)

func (p *Planner) planEnum(typ *plan.Type) {
	// TODO: implement.
}

// isEnumType returns true if the given type declaration looks like an enum type, i.e. it's a basic
// type that's either a string or int-ish, and has at least one constant associated with it.
func isEnumType(typ types.TypeDecl) bool {
	if len(collectExportedConstants(typ.Constants)) == 0 {
		return false
	}

	underlying := types.UnwrapAlias(typ.Underlying)
	if underlying.Kind != types.TypeKindBasic {
		return false
	}

	switch underlying.Name {
	case "string",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"uintptr":
		return true
	default:
		return false
	}
}

func collectExportedConstants(constants map[string]types.ConstantDecl) []types.ConstantDecl {
	out := make([]types.ConstantDecl, 0, len(constants))
	for _, constant := range constants {
		if constant.IsExported {
			out = append(out, constant)
		}
	}
	return out
}
