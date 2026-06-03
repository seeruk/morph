package config_test

import (
	"testing"

	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_DefaultPrecedence(t *testing.T) {
	t.Run("resolves top-level defaults used for implicit mappings", func(t *testing.T) {
		got := resolveConfig(t, precedenceConfig())

		assert.Equal(t, spec.EnumFailureModeZero, got.Defaults.Types.Enum.FailureMode)
		assert.Equal(t, "DefaultForward", got.Defaults.Types.Mappers.Forward.Name)
		assert.Equal(t, spec.PointerOptionalityError, got.Defaults.Types.Optionality.OnNilSourcePointer)
		assert.Equal(t, spec.ValueOptionalityNil, got.Defaults.Types.Optionality.OnZeroSourceValue)
	})

	t.Run("uses package output over top-level output", func(t *testing.T) {
		got := resolveConfig(t, precedenceConfig())

		assert.Equal(t, spec.Output{
			Strategy: spec.OutputStrategySinglePackage,
			Path:     "internal/mapping",
			Package:  "mapping",
			Filename: "mapping.gen.go",
		}, got.Packages[0].Output)
	})

	t.Run("keeps explicit source and target type names", func(t *testing.T) {
		typ := singleResolvedType(t, precedenceConfig())

		assert.Equal(t, "SourceUser", typ.Source)
		assert.Equal(t, "TargetUser", typ.Target)
	})

	t.Run("uses type enum config over package enum defaults", func(t *testing.T) {
		typ := singleResolvedType(t, precedenceConfig())

		assert.Equal(t, spec.EnumFailureModeZero, typ.Enum.FailureMode)
	})

	t.Run("merges package mapper defaults with type mapper overrides", func(t *testing.T) {
		typ := singleResolvedType(t, precedenceConfig())

		assert.Equal(t, "MapUser", typ.Mapper.Name)
		assert.Equal(t, spec.ParameterKindValue, typ.Mapper.Signature.Accepts)
		assert.Equal(t, spec.ParameterKindValue, typ.Mapper.Signature.Returns)
	})

	t.Run("merges package optionality defaults with type optionality overrides", func(t *testing.T) {
		typ := singleResolvedType(t, precedenceConfig())

		assert.Equal(t, spec.PointerOptionalityZero, typ.Optionality.OnNilSourcePointer)
		assert.Equal(t, spec.ValueOptionalityAddress, typ.Optionality.OnZeroSourceValue)
	})

	t.Run("resolves field optionality from type defaults and field overrides", func(t *testing.T) {
		typ := singleResolvedType(t, precedenceConfig())

		assert.Equal(t, spec.Field{
			Target: "DisplayName",
			Optionality: spec.Optionality{
				OnNilSourcePointer: spec.PointerOptionalityError,
				OnZeroSourceValue:  spec.ValueOptionalityAddress,
			},
		}, typ.Struct.Fields["Name"])
	})
}

func TestResolve_BidirectionalExpansion(t *testing.T) {
	t.Run("expands one config type into forward and inverse resolved types", func(t *testing.T) {
		types := resolvedBidirectionalTypes(t)
		require.Len(t, types, 2)

		assert.Equal(t, "SourceRecipe", types[0].Source)
		assert.Equal(t, "TargetRecipe", types[1].Source)
	})

	t.Run("keeps source to target config on the forward mapping", func(t *testing.T) {
		forward := resolvedBidirectionalTypes(t)[0]

		assert.Equal(t, "TargetRecipe", forward.Target)
		assert.Equal(t, "ID", forward.Struct.Fields["RecipeId"].Target)
		assert.Equal(t, map[string]string{"SourceReady": "TargetReady"}, forward.Enum.Values)
	})

	t.Run("inverts struct fields and enum values on the inverse mapping", func(t *testing.T) {
		inverse := resolvedBidirectionalTypes(t)[1]

		assert.Equal(t, "SourceRecipe", inverse.Target)
		assert.Equal(t, "RecipeId", inverse.Struct.Fields["ID"].Target)
		assert.Equal(t, map[string]string{"TargetReady": "SourceReady"}, inverse.Enum.Values)
	})

	t.Run("uses forward and inverse mapper defaults", func(t *testing.T) {
		types := resolvedBidirectionalTypes(t)

		assert.Equal(t, "Map{{ .Source.Package }}{{ .Source.Type }}To{{ .Target.Package }}{{ .Target.Type }}", types[0].Mapper.Name)
		assert.Equal(t, "Map{{ .Source.Package }}{{ .Source.Type }}From{{ .Target.Package }}{{ .Target.Type }}", types[1].Mapper.Name)
	})
}

func TestResolve_DefaultOptionalityUsesNilForZeroSourceValues(t *testing.T) {
	t.Run("resolved top-level defaults use nil", func(t *testing.T) {
		got := resolveConfig(t, minimalConfig())

		assert.Equal(t, spec.ValueOptionalityNil, got.Defaults.Types.Optionality.OnZeroSourceValue)
	})

	t.Run("resolved explicit type optionality uses nil", func(t *testing.T) {
		typ := singleResolvedType(t, minimalConfig())

		assert.Equal(t, spec.ValueOptionalityNil, typ.Optionality.OnZeroSourceValue)
	})
}

func TestResolve_Presets(t *testing.T) {
	t.Run("type preset overrides package preset bidirectionality", func(t *testing.T) {
		got := resolveConfig(t, presetConfig())

		require.Len(t, got.Packages[0].Types, 1)
	})

	t.Run("type preset overrides enum defaults", func(t *testing.T) {
		typ := singleResolvedType(t, presetConfig())

		assert.Equal(t, spec.EnumFailureModeZero, typ.Enum.FailureMode)
	})

	t.Run("type preset overrides mapper defaults", func(t *testing.T) {
		typ := singleResolvedType(t, presetConfig())

		assert.Equal(t, "MapDB", typ.Mapper.Name)
	})

	t.Run("name expands to matching source and target type names", func(t *testing.T) {
		typ := singleResolvedType(t, presetConfig())

		assert.Equal(t, "User", typ.Source)
		assert.Equal(t, "User", typ.Target)
	})
}

func TestResolve_Errors(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		err  string
	}{
		{
			name: "empty mappings",
			err:  "no mappings specified",
		},
		{
			name: "missing package preset",
			cfg: config.Config{
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Preset: "missing",
					Types:  []config.Type{{Name: "User"}},
				}},
			},
			err: `preset not found "missing"`,
		},
		{
			name: "missing type preset",
			cfg: config.Config{
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Types: []config.Type{{
						Name:   "User",
						Preset: "missing",
					}},
				}},
			},
			err: `preset not found "missing"`,
		},
		{
			name: "invalid type naming",
			cfg: config.Config{
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Types: []config.Type{{
						Source: "SourceUser",
					}},
				}},
			},
			err: "either name, or source and target type names are required",
		},
		{
			name: "missing package source",
			cfg: config.Config{
				Packages: []config.Package{{
					Target: "module.test/target",
					Types:  []config.Type{{Name: "User"}},
				}},
			},
			err: "source package is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Resolve(tt.cfg)

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.err)
		})
	}
}

func precedenceConfig() config.Config {
	return config.Config{
		Defaults: config.Defaults{
			Packages: config.PackagesDefaults{
				Types: config.TypesDefaults{
					Enum: &config.EnumDefaults{
						FailureMode: new(spec.EnumFailureModeZero),
					},
					Mappers: &config.MappersDefaults{
						Forward: &config.MapperDefaults{
							Name: new("DefaultForward"),
							Signature: &config.MapperSignatureDefaults{
								Accepts: new(spec.ParameterKindPointer),
								Returns: new(spec.ParameterKindPointer),
							},
						},
					},
					Optionality: &config.OptionalityDefaults{
						OnNilSourcePointer: new(spec.PointerOptionalityError),
						OnZeroSourceValue:  new(spec.ValueOptionalityNil),
					},
				},
				Output: config.Output{
					Strategy: new(spec.OutputStrategySourcePackage),
					Filename: "source.gen.go",
				},
			},
		},
		Packages: []config.Package{{
			Source: "module.test/source",
			Target: "module.test/target",
			Output: config.Output{
				Strategy: new(spec.OutputStrategySinglePackage),
				Path:     "internal/mapping",
				Package:  "mapping",
				Filename: "mapping.gen.go",
			},
			Enum: &config.EnumDefaults{
				FailureMode: new(spec.EnumFailureModeError),
			},
			Mappers: &config.MappersDefaults{
				Forward: &config.MapperDefaults{
					Signature: &config.MapperSignatureDefaults{
						Accepts: new(spec.ParameterKindValue),
					},
				},
			},
			Optionality: &config.OptionalityDefaults{
				OnNilSourcePointer: new(spec.PointerOptionalityZero),
			},
			Types: []config.Type{{
				Source: "SourceUser",
				Target: "TargetUser",
				Enum: &config.Enum{
					FailureMode: new(spec.EnumFailureModeZero),
				},
				Struct: &config.Struct{
					Fields: map[string]config.Field{
						"Name": {
							Target: "DisplayName",
							Optionality: &config.OptionalityDefaults{
								OnNilSourcePointer: new(spec.PointerOptionalityError),
							},
						},
					},
				},
				Mappers: &config.MappersDefaults{
					Forward: &config.MapperDefaults{
						Name: new("MapUser"),
						Signature: &config.MapperSignatureDefaults{
							Returns: new(spec.ParameterKindValue),
						},
					},
				},
				Optionality: &config.OptionalityDefaults{
					OnZeroSourceValue: new(spec.ValueOptionalityAddress),
				},
			}},
		}},
	}
}

func bidirectionalConfig() config.Config {
	return config.Config{
		Packages: []config.Package{{
			Source:        "module.test/source",
			Target:        "module.test/target",
			Bidirectional: new(true),
			Types: []config.Type{{
				Source: "SourceRecipe",
				Target: "TargetRecipe",
				Enum: &config.Enum{
					Values: map[string]string{
						"SourceReady": "TargetReady",
					},
				},
				Struct: &config.Struct{
					Fields: map[string]config.Field{
						"RecipeId": {Target: "ID"},
					},
				},
			}},
		}},
	}
}

func minimalConfig() config.Config {
	return config.Config{
		Packages: []config.Package{{
			Source: "module.test/source",
			Target: "module.test/target",
			Types:  []config.Type{{Name: "User"}},
		}},
	}
}

func presetConfig() config.Config {
	return config.Config{
		Presets: map[string]config.Preset{
			"api": {
				Bidirectional: new(true),
				Mappers: &config.MappersDefaults{
					Forward: &config.MapperDefaults{Name: new("MapAPI")},
					Inverse: &config.MapperDefaults{Name: new("MapAPIInverse")},
				},
			},
			"db": {
				Bidirectional: new(false),
				Enum: &config.EnumDefaults{
					FailureMode: new(spec.EnumFailureModeZero),
				},
				Mappers: &config.MappersDefaults{
					Forward: &config.MapperDefaults{Name: new("MapDB")},
				},
			},
		},
		Packages: []config.Package{{
			Source: "module.test/source",
			Target: "module.test/target",
			Preset: "api",
			Types: []config.Type{{
				Name:   "User",
				Preset: "db",
			}},
		}},
	}
}

func resolveConfig(t *testing.T, cfg config.Config) spec.Spec {
	t.Helper()

	got, err := config.Resolve(cfg)
	require.NoError(t, err)
	require.Len(t, got.Packages, 1)
	return got
}

func singleResolvedType(t *testing.T, cfg config.Config) spec.Type {
	t.Helper()

	got := resolveConfig(t, cfg)
	require.Len(t, got.Packages[0].Types, 1)
	return got.Packages[0].Types[0]
}

func resolvedBidirectionalTypes(t *testing.T) []spec.Type {
	t.Helper()

	got := resolveConfig(t, bidirectionalConfig())
	return got.Packages[0].Types
}
