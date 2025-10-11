package starzelle

/**
 * Starlark utility libraries for starzelle plugins.
 *
 * See starlark/stdlib for standard non-starzelle starlark libraries.
 */

import (
	"fmt"

	common "github.com/aspect-build/aspect-gazelle/common"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	starUtils "github.com/aspect-build/aspect-gazelle/language/orion/starlark/utils"
	"go.starlark.net/starlark"
)

func deprecatedRegisterConfigureExtension(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fmt.Printf("DEPRECATED: 'register_configure_extension' is deprecated, please use 'orion_extension' instead.\n")
	return registerOrionPlugin(t, b, args, kwargs)
}

func registerOrionPlugin(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pluginId starlark.String
	var properties *starlark.Dict
	var prepare, analyze, declare *starlark.Function

	err := starlark.UnpackArgs(
		"orion_extension",
		args,
		kwargs,
		"id", &pluginId,
		"properties?", &properties,
		"prepare?", &prepare,
		"analyze?", &analyze,
		"declare?", &declare,
	)
	if err != nil {
		return nil, err
	}

	err = t.Local(proxyStateKey).(*starzelleState).addPlugin(
		t,
		pluginId,
		properties,
		prepare,
		analyze,
		declare,
	)

	return starlark.None, err
}

func deprecatedRegisterRuleKind(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	fmt.Printf("DEPRECATED: 'register_rule_kind' is deprecated, please use 'gazelle_rule_kind' instead.\n")

	return registerGazelleRuleKind(t, b, args, kwargs)
}

func registerGazelleRuleKind(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var kind starlark.String
	var attributes *starlark.Dict

	err := starlark.UnpackArgs(
		"gazelle_rule_kind",
		args,
		kwargs,
		"name", &kind,
		"attributes?", &attributes,
	)
	if err != nil {
		return nil, err
	}

	err = t.Local(proxyStateKey).(*starzelleState).addKind(t, kind, attributes)
	return starlark.None, err
}

func readQueryFilters(v starlark.Value) ([]string, common.GlobExpr, error) {
	if v == nil {
		return nil, nil, nil
	}

	if filterString, ok := v.(starlark.String); ok {
		s, err := starUtils.ReadString(filterString)
		if err != nil {
			return nil, nil, err
		}
		e, err := common.ParseGlobExpression(s)
		if err != nil {
			return nil, nil, err
		}
		return []string{s}, e, nil
	}

	s, err := starUtils.ReadStringList(v)
	if err != nil {
		return nil, nil, err
	}
	e, err := common.ParseGlobExpressions(s)
	if err != nil {
		return nil, nil, err
	}
	return s, e, nil
}

func newAstQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var query starlark.String
	var filterValue starlark.Value
	var grammarValue starlark.String

	err := starlark.UnpackArgs(
		"AstQuery",
		args,
		kwargs,
		"query", &query,
		"grammar?", &grammarValue,
		"filter??", &filterValue,
	)
	if err != nil {
		return nil, err
	}

	filters, exp, err := readQueryFilters(filterValue)
	if err != nil {
		return nil, err
	}

	return plugin.QueryDefinition{
		Filter:     filters,
		FilterExpr: exp,
		QueryType:  plugin.QueryTypeAst,
		Params: plugin.AstQueryParams{
			Grammar: grammarValue.GoString(),
			Query:   query.GoString(),
		},
	}, nil
}

func newRegexQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expression starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"RegexQuery",
		args,
		kwargs,
		"expression", &expression,
		"filter??", &filterValue,
	)
	if err != nil {
		return nil, err
	}

	filters, exp, err := readQueryFilters(filterValue)
	if err != nil {
		return nil, err
	}

	return plugin.QueryDefinition{
		Filter:     filters,
		FilterExpr: exp,
		QueryType:  plugin.QueryTypeRegex,
		Params:     expression.GoString(),
	}, nil
}

func newRawQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"RawQuery",
		args,
		kwargs,
		"filter??", &filterValue,
	)
	if err != nil {
		return nil, err
	}

	filters, exp, err := readQueryFilters(filterValue)
	if err != nil {
		return nil, err
	}

	return plugin.QueryDefinition{
		Filter:     filters,
		FilterExpr: exp,
		QueryType:  plugin.QueryTypeRaw,
	}, nil
}

func newJsonQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var queryValue starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"JsonQuery",
		args,
		kwargs,
		"query?", &queryValue,
		"filter??", &filterValue,
	)
	if err != nil {
		return nil, err
	}

	filters, exp, err := readQueryFilters(filterValue)
	if err != nil {
		return nil, err
	}

	return plugin.QueryDefinition{
		Filter:     filters,
		FilterExpr: exp,
		QueryType:  plugin.QueryTypeJson,
		Params:     queryValue.GoString(),
	}, nil
}

func newYamlQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var queryValue starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"YamlQuery",
		args,
		kwargs,
		"query?", &queryValue,
		"filter??", &filterValue,
	)
	if err != nil {
		return nil, err
	}

	filters, exp, err := readQueryFilters(filterValue)
	if err != nil {
		return nil, err
	}

	return plugin.QueryDefinition{
		Filter:     filters,
		FilterExpr: exp,
		QueryType:  plugin.QueryTypeYaml,
		Params:     queryValue.GoString(),
	}, nil
}

func newSourceExtensions(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	exts, err := starUtils.ReadStringTuple(args)
	if err != nil {
		return nil, err
	}
	return plugin.NewSourceExtensionsFilter(exts).(starlark.Value), nil
}

func newSourceGlobs(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	globs, err := starUtils.ReadTuple(args, starUtils.ReadString)
	if err != nil {
		return nil, err
	}
	f, err := plugin.NewSourceGlobFilter(globs)
	if err != nil {
		return nil, err
	}
	return f.(starlark.Value), nil
}

func newSourceFiles(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	files, err := starUtils.ReadStringTuple(args)
	if err != nil {
		return nil, err
	}
	return plugin.NewSourceFileFilter(files).(starlark.Value), nil
}

func newPrepareResult(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var queriesValue *starlark.Dict
	var sourcesValue starlark.Value

	err := starlark.UnpackArgs(
		"PrepareResult",
		args,
		kwargs,
		"sources", &sourcesValue,
		"queries??", &queriesValue,
	)
	if err != nil {
		return nil, err
	}

	queries := make(plugin.NamedQueries)
	if queriesValue != nil {
		iter := queriesValue.Iterate()
		defer iter.Done()

		var k starlark.Value
		for iter.Next(&k) {
			v, _, _ := queriesValue.Get(k)

			qd, isQd := v.(plugin.QueryDefinition)
			if !isQd {
				return nil, fmt.Errorf("'queries' %v (%T) is not a QueryDefinition", v, v)
			}

			queries[k.(starlark.String).GoString()] = qd
		}
	}

	var sources map[string][]plugin.SourceFilter
	if sourcesValue != nil {
		// Allow source values as a flat list or a map of lists
		if sourceDict, isDict := (sourcesValue).(*starlark.Dict); isDict {
			sources, err = starUtils.ReadMap2(sourceDict, readSourceFilterEntry)
			if err != nil {
				return nil, err
			}
		} else {
			g, err := readSourceFilterEntry(sourcesValue)
			if err != nil {
				return nil, err
			}
			sources = map[string][]plugin.SourceFilter{
				plugin.DeclareTargetsContextDefaultGroup: g,
			}
		}
	}

	return plugin.PrepareResult{
		Sources: sources,
		Queries: queries,
	}, nil
}

func readSourceFilterEntry(v starlark.Value) ([]plugin.SourceFilter, error) {
	if list, isList := v.(*starlark.List); isList {
		return starUtils.ReadList(list, readSourceFilter)
	} else {
		v, err := readSourceFilter(v)
		if err != nil {
			return nil, err
		}
		return []plugin.SourceFilter{v}, nil
	}
}

func readSourceFilter(v starlark.Value) (plugin.SourceFilter, error) {
	f, isF := v.(plugin.SourceFilter)
	if !isF {
		return nil, fmt.Errorf("'sources' %v (%T) is not a SourceFilter", f, f)
	}
	return f, nil
}

func newImport(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var id, provider, from starlark.String
	var optional starlark.Bool

	err := starlark.UnpackArgs(
		"Import",
		args,
		kwargs,
		"id", &id,
		"provider", &provider,
		"src?", &from,
		"optional?", &optional,
	)
	if err != nil {
		return nil, err
	}

	if id.GoString() == "" || provider.GoString() == "" {
		return nil, fmt.Errorf("import id and provider cannot be empty")
	}

	return plugin.TargetImport{
		Symbol: plugin.Symbol{
			Id:       id.GoString(),
			Provider: provider.GoString(),
		},
		Optional: bool(optional.Truth()),
		From:     from.GoString(),
	}, nil
}

func newSymbol(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var id, provider starlark.String

	err := starlark.UnpackArgs(
		"Symbol",
		args,
		kwargs,
		"id", &id,
		"provider", &provider,
	)
	if err != nil {
		return nil, err
	}

	return plugin.Symbol{
		Id:       id.GoString(),
		Provider: provider.GoString(),
	}, nil
}

func newLabel(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var repo, pkg, name starlark.String

	err := starlark.UnpackArgs(
		"Label",
		args,
		kwargs,
		"repo?", &repo,
		"pkg?", &pkg,
		"name", &name,
	)
	if err != nil {
		return nil, err
	}

	return plugin.Label{
		Repo: repo.GoString(),
		Pkg:  pkg.GoString(),
		Name: name.GoString(),
	}, nil
}

func newProperty(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var propType starlark.String
	var propDefault starlark.Value = starlark.None

	err := starlark.UnpackArgs(
		"Property",
		args,
		kwargs,
		"type", &propType,
		"default?", &propDefault,
	)
	if err != nil {
		return nil, err
	}

	defaultValue, err := starUtils.Read(propDefault)
	if err != nil {
		return nil, err
	}

	return plugin.Property{
		PropertyType: propType.GoString(),
		Default:      defaultValue,
	}, nil
}

var aspectModule = starUtils.CreateModule(
	"aspect",
	map[string]starUtils.ModuleFunction{
		"register_configure_extension": deprecatedRegisterConfigureExtension,
		"register_rule_kind":           deprecatedRegisterRuleKind,
		"orion_extension":              registerOrionPlugin,
		"gazelle_rule_kind":            registerGazelleRuleKind,
		"AstQuery":                     newAstQuery,
		"RegexQuery":                   newRegexQuery,
		"RawQuery":                     newRawQuery,
		"JsonQuery":                    newJsonQuery,
		"YamlQuery":                    newYamlQuery,
		"PrepareResult":                newPrepareResult,
		"Import":                       newImport,
		"Symbol":                       newSymbol,
		"Label":                        newLabel,
		"Property":                     newProperty,
		"SourceExtensions":             newSourceExtensions,
		"SourceGlobs":                  newSourceGlobs,
		"SourceFiles":                  newSourceFiles,
	},
	map[string]starlark.Value{},
)
