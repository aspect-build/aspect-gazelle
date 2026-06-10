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

func alwaysMatchExpr(string) bool {
	return true
}

func readQueryBase(v starlark.Value) (plugin.QueryBase, error) {
	if v == nil {
		return plugin.QueryBase{FilterExpr: alwaysMatchExpr}, nil
	}

	if filterString, ok := v.(starlark.String); ok {
		s, err := starUtils.ReadString(filterString)
		if err != nil {
			return plugin.QueryBase{}, err
		}
		e, err := common.ParseGlobExpression(s)
		if err != nil {
			return plugin.QueryBase{}, err
		}
		return plugin.QueryBase{Filter: []string{s}, FilterExpr: e}, nil
	}

	s, err := starUtils.ReadStringList(v)
	if err != nil {
		return plugin.QueryBase{}, err
	}
	e, err := common.ParseGlobExpressions(s)
	if err != nil {
		return plugin.QueryBase{}, err
	}
	return plugin.QueryBase{Filter: s, FilterExpr: e}, nil
}

// readContentFilter parses the optional `content_filter` arg, returning the
// raw pattern and its compiled content matcher. Parsed here so authors get a
// regex error at plugin load.
func readContentFilter(v starlark.String) (string, common.BytesMatcher, error) {
	s := v.GoString()
	exp, err := common.ParseMatcher(s)
	if err != nil {
		return "", nil, fmt.Errorf("`content_filter` is not a valid regex %q: %w", s, err)
	}
	return s, exp, nil
}

func newAstQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var query starlark.String
	var contentFilterValue starlark.String
	var filterValue starlark.Value
	var grammarValue starlark.String

	err := starlark.UnpackArgs(
		"AstQuery",
		args,
		kwargs,
		"query", &query,
		"grammar?", &grammarValue,
		"filter??", &filterValue,
		"content_filter??", &contentFilterValue,
	)
	if err != nil {
		return nil, err
	}

	base, err := readQueryBase(filterValue)
	if err != nil {
		return nil, err
	}

	base.ContentFilter, base.ContentFilterExpr, err = readContentFilter(contentFilterValue)
	if err != nil {
		return nil, err
	}

	return &plugin.AstQuery{
		QueryBase: base,
		Grammar:   grammarValue.GoString(),
		Query:     query.GoString(),
	}, nil
}

func newRegexQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var expression starlark.String
	var contentFilterValue starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"RegexQuery",
		args,
		kwargs,
		"expression", &expression,
		"filter??", &filterValue,
		"content_filter??", &contentFilterValue,
	)
	if err != nil {
		return nil, err
	}

	base, err := readQueryBase(filterValue)
	if err != nil {
		return nil, err
	}

	base.ContentFilter, base.ContentFilterExpr, err = readContentFilter(contentFilterValue)
	if err != nil {
		return nil, err
	}

	return plugin.NewRegexQuery(base, expression.GoString())
}

func newRawQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var contentFilterValue starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"RawQuery",
		args,
		kwargs,
		"filter??", &filterValue,
		"content_filter??", &contentFilterValue,
	)
	if err != nil {
		return nil, err
	}

	base, err := readQueryBase(filterValue)
	if err != nil {
		return nil, err
	}

	base.ContentFilter, base.ContentFilterExpr, err = readContentFilter(contentFilterValue)
	if err != nil {
		return nil, err
	}

	return &plugin.RawQuery{
		QueryBase: base,
	}, nil
}

func newJsonQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var queryValue starlark.String
	var contentFilterValue starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"JsonQuery",
		args,
		kwargs,
		"query?", &queryValue,
		"filter??", &filterValue,
		"content_filter??", &contentFilterValue,
	)
	if err != nil {
		return nil, err
	}

	base, err := readQueryBase(filterValue)
	if err != nil {
		return nil, err
	}

	base.ContentFilter, base.ContentFilterExpr, err = readContentFilter(contentFilterValue)
	if err != nil {
		return nil, err
	}

	return &plugin.JsonQuery{
		QueryBase: base,
		Query:     queryValue.GoString(),
	}, nil
}

func newYamlQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var queryValue starlark.String
	var contentFilterValue starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"YamlQuery",
		args,
		kwargs,
		"query?", &queryValue,
		"filter??", &filterValue,
		"content_filter??", &contentFilterValue,
	)
	if err != nil {
		return nil, err
	}

	base, err := readQueryBase(filterValue)
	if err != nil {
		return nil, err
	}

	base.ContentFilter, base.ContentFilterExpr, err = readContentFilter(contentFilterValue)
	if err != nil {
		return nil, err
	}

	return &plugin.YamlQuery{
		QueryBase: base,
		Query:     queryValue.GoString(),
	}, nil
}

func newTomlQuery(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var queryValue starlark.String
	var contentFilterValue starlark.String
	var filterValue starlark.Value

	err := starlark.UnpackArgs(
		"TomlQuery",
		args,
		kwargs,
		"query?", &queryValue,
		"filter??", &filterValue,
		"content_filter??", &contentFilterValue,
	)
	if err != nil {
		return nil, err
	}

	base, err := readQueryBase(filterValue)
	if err != nil {
		return nil, err
	}

	base.ContentFilter, base.ContentFilterExpr, err = readContentFilter(contentFilterValue)
	if err != nil {
		return nil, err
	}

	return &plugin.TomlQuery{
		QueryBase: base,
		Query:     queryValue.GoString(),
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
	// Include patterns are passed positionally, either variadic or as a single
	// list aligning with bazel glob(); the optional "exclude" keyword argument
	// takes a list of patterns to subtract from the matched set.
	var globs []string
	var err error
	if len(args) == 1 {
		if _, isList := args[0].(*starlark.List); isList {
			globs, err = starUtils.ReadStringList(args[0])
		}
	}
	if globs == nil && err == nil {
		globs, err = starUtils.ReadStringTuple(args)
	}
	if err != nil {
		return nil, err
	}

	var excludes []string
	for _, kwarg := range kwargs {
		name, err := starUtils.ReadString(kwarg[0])
		if err != nil {
			return nil, err
		}
		switch name {
		case "exclude":
			excludes, err = starUtils.ReadStringList(kwarg[1])
			if err != nil {
				return nil, fmt.Errorf("exclude: %w", err)
			}
		default:
			return nil, fmt.Errorf("unexpected keyword argument %q", name)
		}
	}

	f, err := plugin.NewSourceGlobFilter(globs, excludes)
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

	var queries plugin.NamedQueries
	if queriesValue != nil {
		queries = make(plugin.NamedQueries, queriesValue.Len())

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
	var optional, ancestor, multiple starlark.Bool

	err := starlark.UnpackArgs(
		"Import",
		args,
		kwargs,
		"id", &id,
		"provider", &provider,
		"src?", &from,
		"optional?", &optional,
		"ancestor?", &ancestor,
		"multiple?", &multiple,
	)
	if err != nil {
		return nil, err
	}

	if id.GoString() == "" || provider.GoString() == "" {
		return nil, fmt.Errorf("import id and provider cannot be empty")
	}

	// ancestor (find the nearest single provider by walking up) and multiple
	// (collect every provider) pull in opposite directions; their combination has
	// no coherent meaning, so reject it rather than resolve it surprisingly.
	if bool(ancestor.Truth()) && bool(multiple.Truth()) {
		return nil, fmt.Errorf("import cannot be both ancestor and multiple")
	}

	return plugin.TargetImport{
		Symbol: plugin.Symbol{
			Id:       id.GoString(),
			Provider: provider.GoString(),
		},
		Optional: bool(optional.Truth()),
		Ancestor: bool(ancestor.Truth()),
		Multiple: bool(multiple.Truth()),
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
		"TomlQuery":                    newTomlQuery,
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
