package morph

import (
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
)

func (p *Planner) planStruct(typ *plan.Type) {
	// TODO: implement.
}

// isStructType returns true if the given type declaration looks like a struct type, i.e. its
// underlying type is a struct.
func isStructType(typ types.TypeDecl) bool {
	return types.UnwrapAlias(typ.Underlying).Kind == types.TypeKindStruct
}
