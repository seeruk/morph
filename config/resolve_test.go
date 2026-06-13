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

	t.Run("resolves property optionality from type defaults and property overrides", func(t *testing.T) {
		typ := singleResolvedType(t, precedenceConfig())

		require.Len(t, typ.Struct.Properties, 1)
		assert.Equal(t, spec.Property{
			Source: "Name",
			Target: "DisplayName",
			Optionality: spec.Optionality{
				OnNilSourcePointer: spec.PointerOptionalityError,
				OnZeroSourceValue:  spec.ValueOptionalityAddress,
			},
			Conversions: spec.ConversionsPolicy{
				Enabled: true,
			},
		}, typ.Struct.Properties[0])
	})
}

func TestResolve_StructOmissions(t *testing.T) {
	t.Run("resolves and dedupes omissions", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Packages[0].Types[0].Struct = &config.Struct{Omit: config.StructOmissions{
			Both:   []string{"Shared", "Shared", "Matched"},
			Source: []string{"Legacy", "Internal", "Legacy"},
			Target: []string{"CreatedAt", "CreatedAt"},
		}}

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.StructOmissions{
			Both:   []string{"Matched", "Shared"},
			Source: []string{"Internal", "Legacy"},
			Target: []string{"CreatedAt"},
		}, typ.Struct.Omit)
	})

	t.Run("swaps omissions for inverse mappings", func(t *testing.T) {
		cfg := bidirectionalConfig()
		cfg.Packages[0].Types[0].Struct.Omit = config.StructOmissions{
			Both:   []string{"Matched"},
			Source: []string{"ForwardSource"},
			Target: []string{"ForwardTarget"},
		}

		types := resolveConfig(t, cfg).Packages[0].Types
		require.Len(t, types, 2)

		assert.Equal(t, spec.StructOmissions{
			Both:   []string{"Matched"},
			Source: []string{"ForwardSource"},
			Target: []string{"ForwardTarget"},
		}, types[0].Struct.Omit)
		assert.Equal(t, spec.StructOmissions{
			Both:   []string{"Matched"},
			Source: []string{"ForwardTarget"},
			Target: []string{"ForwardSource"},
		}, types[1].Struct.Omit)
	})
}

func TestResolve_EnumPatternDefaults(t *testing.T) {
	t.Run("uses global enum default patterns", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Defaults.Packages.Types.Enum = &config.EnumDefaults{
			Patterns: &config.EnumPatterns{
				Source: "DefaultSource",
				Target: "DefaultTarget",
			},
		}

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumPatterns{Source: "DefaultSource", Target: "DefaultTarget"}, typ.Enum.Patterns)
	})

	t.Run("uses package preset enum patterns", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Presets = map[string]config.Preset{
			"api": {Enum: &config.EnumDefaults{Patterns: &config.EnumPatterns{
				Source: "PackagePresetSource",
				Target: "PackagePresetTarget",
			}}},
		}
		cfg.Packages[0].Preset = "api"

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumPatterns{Source: "PackagePresetSource", Target: "PackagePresetTarget"}, typ.Enum.Patterns)
	})

	t.Run("type preset overrides package preset enum patterns", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Presets = map[string]config.Preset{
			"api": {Enum: &config.EnumDefaults{Patterns: &config.EnumPatterns{
				Source: "PackagePresetSource",
				Target: "PackagePresetTarget",
			}}},
			"db": {Enum: &config.EnumDefaults{Patterns: &config.EnumPatterns{
				Source: "TypePresetSource",
				Target: "TypePresetTarget",
			}}},
		}
		cfg.Packages[0].Preset = "api"
		cfg.Packages[0].Types[0].Preset = "db"

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumPatterns{Source: "TypePresetSource", Target: "TypePresetTarget"}, typ.Enum.Patterns)
	})

	t.Run("package enum patterns override global defaults", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Defaults.Packages.Types.Enum = &config.EnumDefaults{
			Patterns: &config.EnumPatterns{Source: "DefaultSource", Target: "DefaultTarget"},
		}
		cfg.Packages[0].Enum = &config.EnumDefaults{
			Patterns: &config.EnumPatterns{Source: "PackageSource", Target: "PackageTarget"},
		}

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumPatterns{Source: "PackageSource", Target: "PackageTarget"}, typ.Enum.Patterns)
	})

	t.Run("type enum patterns override inherited defaults", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Defaults.Packages.Types.Enum = &config.EnumDefaults{
			Patterns: &config.EnumPatterns{Source: "DefaultSource", Target: "DefaultTarget"},
		}
		cfg.Packages[0].Types[0].Enum = &config.Enum{
			Patterns: &config.EnumPatterns{Source: "TypeSource", Target: "TypeTarget"},
		}

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumPatterns{Source: "TypeSource", Target: "TypeTarget"}, typ.Enum.Patterns)
	})

	t.Run("partial enum pattern overrides inherit the unspecified side", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Defaults.Packages.Types.Enum = &config.EnumDefaults{
			Patterns: &config.EnumPatterns{Source: "DefaultSource", Target: "DefaultTarget"},
		}
		cfg.Packages[0].Enum = &config.EnumDefaults{
			Patterns: &config.EnumPatterns{Source: "PackageSource"},
		}
		cfg.Packages[0].Types[0].Enum = &config.Enum{
			Patterns: &config.EnumPatterns{Target: "TypeTarget"},
		}

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumPatterns{Source: "PackageSource", Target: "TypeTarget"}, typ.Enum.Patterns)
	})
}

func TestResolve_EnumFallback(t *testing.T) {
	t.Run("resolves forward and inverse fallback values directionally", func(t *testing.T) {
		fallback := spec.EnumFailureModeFallback
		cfg := bidirectionalConfig()
		cfg.Packages[0].Types[0].Enum.FailureMode = &fallback
		cfg.Packages[0].Types[0].Enum.Fallback = &config.EnumFallback{
			Forward: "TargetUnknown",
			Inverse: "SourceUnknown",
		}

		types := resolvedBidirectionalTypes(t, cfg)
		require.Len(t, types, 2)

		assert.Equal(t, spec.EnumFailureModeFallback, types[0].Enum.FailureMode)
		assert.Equal(t, "TargetUnknown", types[0].Enum.FallbackValue)
		assert.Equal(t, map[string]string{"SourceReady": "TargetReady"}, types[0].Enum.Values)

		assert.Equal(t, spec.EnumFailureModeFallback, types[1].Enum.FailureMode)
		assert.Equal(t, "SourceUnknown", types[1].Enum.FallbackValue)
		assert.Equal(t, map[string]string{"TargetReady": "SourceReady"}, types[1].Enum.Values)
	})

	t.Run("inherits fallback failure mode from defaults while type supplies fallback value", func(t *testing.T) {
		fallback := spec.EnumFailureModeFallback
		cfg := minimalConfig()
		cfg.Defaults.Packages.Types.Enum = &config.EnumDefaults{FailureMode: &fallback}
		cfg.Packages[0].Types[0].Enum = &config.Enum{
			Fallback: &config.EnumFallback{Forward: "TargetUnknown"},
		}

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumFailureModeFallback, typ.Enum.FailureMode)
		assert.Equal(t, "TargetUnknown", typ.Enum.FallbackValue)
	})

	t.Run("inherits fallback failure mode from presets while type supplies fallback value", func(t *testing.T) {
		fallback := spec.EnumFailureModeFallback
		cfg := minimalConfig()
		cfg.Presets = map[string]config.Preset{
			"api": {Enum: &config.EnumDefaults{FailureMode: &fallback}},
		}
		cfg.Packages[0].Preset = "api"
		cfg.Packages[0].Types[0].Enum = &config.Enum{
			Fallback: &config.EnumFallback{Forward: "TargetUnknown"},
		}

		typ := singleResolvedType(t, cfg)

		assert.Equal(t, spec.EnumFailureModeFallback, typ.Enum.FailureMode)
		assert.Equal(t, "TargetUnknown", typ.Enum.FallbackValue)
	})
}

func TestResolve_BidirectionalExpansion(t *testing.T) {
	t.Run("expands one config type into forward and inverse resolved types", func(t *testing.T) {
		types := resolvedBidirectionalTypes(t)
		require.Len(t, types, 2)

		assert.Equal(t, "SourceRecipe", types[0].Source)
		assert.Equal(t, "TargetRecipe", types[1].Source)
	})

	t.Run("swaps package context for inverse mappings", func(t *testing.T) {
		types := resolvedBidirectionalTypes(t)
		require.Len(t, types, 2)

		assert.Equal(t, "module.test/source", types[0].SourcePackage)
		assert.Equal(t, "module.test/target", types[0].TargetPackage)
		assert.Equal(t, "module.test/target", types[1].SourcePackage)
		assert.Equal(t, "module.test/source", types[1].TargetPackage)
	})

	t.Run("keeps source to target config on the forward mapping", func(t *testing.T) {
		forward := resolvedBidirectionalTypes(t)[0]

		assert.Equal(t, "TargetRecipe", forward.Target)
		require.Len(t, forward.Struct.Properties, 1)
		assert.Equal(t, "RecipeId", forward.Struct.Properties[0].Source)
		assert.Equal(t, "ID", forward.Struct.Properties[0].Target)
		assert.Equal(t, map[string]string{"SourceReady": "TargetReady"}, forward.Enum.Values)
	})

	t.Run("inverts struct properties and enum values on the inverse mapping", func(t *testing.T) {
		inverse := resolvedBidirectionalTypes(t)[1]

		assert.Equal(t, "SourceRecipe", inverse.Target)
		require.Len(t, inverse.Struct.Properties, 1)
		assert.Equal(t, "ID", inverse.Struct.Properties[0].Source)
		assert.Equal(t, "RecipeId", inverse.Struct.Properties[0].Target)
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

func TestResolve_Conversions(t *testing.T) {
	userID := spec.TypeRef{ImportPath: "module.test/domain", Name: "UserID"}
	apiUserID := spec.TypeRef{ImportPath: "module.test/api", Name: "UserID"}
	stringType := spec.TypeRef{Name: "string"}

	t.Run("expands grouped targets directionally", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Conversions = []config.Conversion{{
			Source:  userID,
			Targets: []spec.TypeRef{stringType, apiUserID},
		}}

		got := resolveConfig(t, cfg)

		assert.Equal(t, []spec.Conversion{
			{Source: userID, Target: stringType},
			{Source: userID, Target: apiUserID},
		}, got.Conversions)
	})

	t.Run("expands bidirectional targets", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Conversions = []config.Conversion{{
			Source:        userID,
			Targets:       []spec.TypeRef{stringType},
			Bidirectional: true,
		}}

		got := resolveConfig(t, cfg)

		assert.Equal(t, []spec.Conversion{
			{Source: userID, Target: stringType},
			{Source: stringType, Target: userID},
		}, got.Conversions)
	})

	t.Run("deduplicates expanded pairs", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Conversions = []config.Conversion{
			{Source: userID, Targets: []spec.TypeRef{stringType}},
			{Source: userID, Targets: []spec.TypeRef{stringType}},
		}

		got := resolveConfig(t, cfg)

		assert.Equal(t, []spec.Conversion{{Source: userID, Target: stringType}}, got.Conversions)
	})

	t.Run("defaults to enabled policy", func(t *testing.T) {
		typ := singleResolvedType(t, minimalConfig())

		assert.True(t, typ.Conversions.Enabled)
	})

	t.Run("uses configured defaults", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Defaults.Packages.Types.Conversions = &config.ConversionsDefaults{
			Enabled: new(false),
		}

		typ := singleResolvedType(t, cfg)

		assert.False(t, typ.Conversions.Enabled)
	})

	t.Run("uses package presets", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Presets = map[string]config.Preset{
			"api": {Conversions: &config.ConversionsDefaults{Enabled: new(false)}},
		}
		cfg.Packages[0].Preset = "api"

		typ := singleResolvedType(t, cfg)

		assert.False(t, typ.Conversions.Enabled)
	})

	t.Run("uses package policy", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Packages[0].Conversions = &config.ConversionsDefaults{
			Enabled: new(false),
		}

		typ := singleResolvedType(t, cfg)

		assert.False(t, typ.Conversions.Enabled)
	})

	t.Run("uses type policy", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Packages[0].Conversions = &config.ConversionsDefaults{
			Enabled: new(false),
		}
		cfg.Packages[0].Types[0].Conversions = &config.ConversionsDefaults{
			Enabled: new(true),
		}

		typ := singleResolvedType(t, cfg)

		assert.True(t, typ.Conversions.Enabled)
	})

	t.Run("uses property policy", func(t *testing.T) {
		cfg := minimalConfig()
		cfg.Packages[0].Types[0].Conversions = &config.ConversionsDefaults{
			Enabled: new(false),
		}
		cfg.Packages[0].Types[0].Struct = &config.Struct{
			Properties: []config.Property{{
				Name:        "ID",
				Conversions: &config.ConversionsDefaults{Enabled: new(true)},
			}},
		}

		typ := singleResolvedType(t, cfg)

		require.Len(t, typ.Struct.Properties, 1)
		assert.True(t, typ.Struct.Properties[0].Conversions.Enabled)
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

func TestResolve_Callables(t *testing.T) {
	t.Run("resolves scoped callables in priority order", func(t *testing.T) {
		got := singleResolvedType(t, callableConfig())

		assert.Equal(t, []spec.PrioritizedCallables{
			{Priority: spec.CallablePriorityType, Callables: []spec.CallableRef{callableRef("type")}},
			{Priority: spec.CallablePriorityTypePreset, Callables: []spec.CallableRef{callableRef("type_preset")}},
			{Priority: spec.CallablePriorityPackage, Callables: []spec.CallableRef{callableRef("package")}},
			{Priority: spec.CallablePriorityPackagePreset, Callables: []spec.CallableRef{callableRef("package_preset")}},
			{Priority: spec.CallablePriorityDefaults, Callables: []spec.CallableRef{callableRef("defaults")}},
		}, got.Callables)
	})

	t.Run("resolves property callables directionally", func(t *testing.T) {
		types := resolvedBidirectionalTypes(t, callableFieldConfig())
		require.Len(t, types, 2)
		require.Len(t, types[0].Struct.Properties, 1)
		require.Len(t, types[1].Struct.Properties, 1)

		assert.Equal(t, callableRef("field_forward"), *types[0].Struct.Properties[0].Callable)
		assert.Equal(t, callableRef("field_inverse"), *types[1].Struct.Properties[0].Callable)
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
		{
			name: "missing conversion source",
			cfg: config.Config{
				Conversions: []config.Conversion{{
					Targets: []spec.TypeRef{{Name: "string"}},
				}},
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Types:  []config.Type{{Name: "User"}},
				}},
			},
			err: "source is required",
		},
		{
			name: "missing conversion targets",
			cfg: config.Config{
				Conversions: []config.Conversion{{
					Source: spec.TypeRef{ImportPath: "module.test/source", Name: "UserID"},
				}},
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Types:  []config.Type{{Name: "User"}},
				}},
			},
			err: "at least one target is required",
		},
		{
			name: "invalid property naming",
			cfg: config.Config{
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Types: []config.Type{{
						Name: "User",
						Struct: &config.Struct{Properties: []config.Property{{
							Source: "Name",
						}}},
					}},
				}},
			},
			err: "either name, or source and target property names are required",
		},
		{
			name: "property name combined with target",
			cfg: config.Config{
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Types: []config.Type{{
						Name: "User",
						Struct: &config.Struct{Properties: []config.Property{{
							Name:   "Name",
							Target: "DisplayName",
						}}},
					}},
				}},
			},
			err: "name cannot be combined with source or target",
		},
		{
			name: "duplicate properties",
			cfg: config.Config{
				Packages: []config.Package{{
					Source: "module.test/source",
					Target: "module.test/target",
					Types: []config.Type{{
						Name: "User",
						Struct: &config.Struct{Properties: []config.Property{
							{Source: "ID", Target: "Name"},
							{Source: "ID", Target: "DisplayName"},
							{Source: "Code", Target: "Name"},
							{Source: "Code", Target: "CodeName"},
							{Source: "Other", Target: "Name"},
						}},
					}},
				}},
			},
			err: `duplicate struct property mappings: source property "Code" appears in properties[2], properties[3]; source property "ID" appears in properties[0], properties[1]; target property "Name" appears in properties[0], properties[2], properties[4]`,
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
					Properties: []config.Property{{
						Source: "Name",
						Target: "DisplayName",
						Optionality: &config.OptionalityDefaults{
							OnNilSourcePointer: new(spec.PointerOptionalityError),
						},
					}},
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
					Properties: []config.Property{{Source: "RecipeId", Target: "ID"}},
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

func callableConfig() config.Config {
	return config.Config{
		Defaults: config.Defaults{
			Packages: config.PackagesDefaults{
				Types: config.TypesDefaults{
					Callables: []spec.CallableRef{callableRef("defaults")},
				},
			},
		},
		Presets: map[string]config.Preset{
			"api": {Callables: []spec.CallableRef{callableRef("package_preset")}},
			"db":  {Callables: []spec.CallableRef{callableRef("type_preset")}},
		},
		Packages: []config.Package{{
			Source:    "module.test/source",
			Target:    "module.test/target",
			Preset:    "api",
			Callables: []spec.CallableRef{callableRef("package")},
			Types: []config.Type{{
				Name:      "User",
				Preset:    "db",
				Callables: []spec.CallableRef{callableRef("type")},
			}},
		}},
	}
}

func callableFieldConfig() config.Config {
	forward := callableRef("field_forward")
	inverse := callableRef("field_inverse")
	cfg := bidirectionalConfig()
	cfg.Packages[0].Types[0].Struct.Properties[0] = config.Property{
		Source: "RecipeId",
		Target: "ID",
		Callable: &config.PropertyCallable{
			Forward: &forward,
			Inverse: &inverse,
		},
	}
	return cfg
}

func callableRef(name string) spec.CallableRef {
	return spec.CallableRef{
		ImportPath: "module.test/callables",
		Name:       name,
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

func resolvedBidirectionalTypes(t *testing.T, cfgs ...config.Config) []spec.Type {
	t.Helper()

	cfg := bidirectionalConfig()
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	got := resolveConfig(t, cfg)
	return got.Packages[0].Types
}
