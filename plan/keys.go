package plan

import (
	"github.com/seeruk/morph/spec"
)

// TypePairKey returns a combination of the keys of two given TypeRef. This key doesn't need to be
// recreated here, we just take the "cached" key on the TypeRef.
func TypePairKey(source TypeRef, target TypeRef) string {
	return source.Key + "->" + target.Key
}

// SignatureKey returns a string that identifies the signature structure of a mapper function.
// Typically, this will be used in combination with another key function to uniquely identify a
// mapper.
func SignatureKey(signature spec.MapperSignature) string {
	return signature.Accepts.String() + "->" + signature.Returns.String()
}

// TypeMapperKey returns a key that uniquely identifies an individual mapper function. This is
// particularly useful for caching planned mappers and looking them up again later, as the types
// used to create this key are unfortunately not able to be comparable (e.g. would contain slices).
func TypeMapperKey(source TypeRef, target TypeRef, signature spec.MapperSignature) string {
	return TypePairKey(source, target) + "|" + SignatureKey(signature)
}
