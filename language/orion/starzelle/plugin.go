package starzelle

/**
 * A proxy into a starzelle plugin file.
 */

import (
	"errors"
	"fmt"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	starEval "github.com/aspect-build/aspect-gazelle/language/orion/starlark"
	starUtils "github.com/aspect-build/aspect-gazelle/language/orion/starlark/utils"
	"go.starlark.net/starlark"
)

var proxyStateKey = "$starzelleState$"

var EmptyPrepareResult = plugin.PrepareResult{
	Sources: make(map[string][]plugin.SourceFilter),
	Queries: plugin.NamedQueries{},
}

var EmptyDeclareTargetsResult = plugin.DeclareTargetsResult{}

type starzelleState struct {
	pluginPath string
	host       plugin.PluginHost
}

func LoadProxy(host plugin.PluginHost, pluginDir, pluginPath string) error {
	BazelLog.Infof("Evaluate orion plugin: %q", pluginPath)

	state := starzelleState{
		pluginPath: pluginPath,
		host:       host,
	}
	evalState := make(map[string]interface{})
	evalState[proxyStateKey] = &state

	libs := starlark.StringDict{
		"aspect": aspectModule,
	}

	_, err := starEval.Eval(pluginDir, pluginPath, libs, evalState)
	if err != nil {
		return err
	}

	return nil
}

func (s *starzelleState) addKind(_ *starlark.Thread, name starlark.String, attributes *starlark.Dict) error {
	pluginKind, err := readRuleKind(name, attributes)
	if err != nil {
		return fmt.Errorf("failed to read rule kind %q: %w", name.GoString(), err)
	}
	s.host.AddKind(pluginKind)
	return nil
}

func (s *starzelleState) addPlugin(t *starlark.Thread, pluginId starlark.String, properties *starlark.Dict, prepare, analyze, declare *starlark.Function) error {
	var pluginProperties map[string]plugin.Property
	var err error

	if properties != nil {
		pluginProperties, err = starUtils.ReadMap(properties, readProperty)
		if err != nil {
			return fmt.Errorf("failed to read plugin properties for %q: %w", pluginId.GoString(), err)
		}
	}

	// A thread is created for each plugin to run in.
	pluginThread := &starlark.Thread{
		Name:  fmt.Sprintf("%s-%s", t.Name, pluginId.GoString()),
		Load:  t.Load,
		Print: t.Print,
	}

	s.host.AddPlugin(starzellePluginProxy{
		t:          pluginThread,
		name:       pluginId.GoString(),
		pluginPath: s.pluginPath,
		properties: pluginProperties,
		prepare:    prepare,
		analyze:    analyze,
		declare:    declare,
	})

	return nil
}

// A plugin implementation loaded via starlark and proxying
// to starlark functions.
var _ plugin.Plugin = (*starzellePluginProxy)(nil)

type starzellePluginProxy struct {
	name                      string
	pluginPath                string
	properties                map[string]plugin.Property
	prepare, analyze, declare *starlark.Function

	// The thread this plugin is running in.
	t *starlark.Thread
}

func (p starzellePluginProxy) Name() string {
	return p.name
}

func (p starzellePluginProxy) Properties() map[string]plugin.Property {
	return p.properties
}

func (p starzellePluginProxy) Prepare(ctx plugin.PrepareContext) plugin.PrepareResult {
	if p.prepare == nil {
		return EmptyPrepareResult
	}

	v, err := starlark.Call(p.t, p.prepare, starlark.Tuple{ctx}, starUtils.EmptyKwArgs)
	if err != nil {
		errStr := starUtils.ErrorStr(fmt.Sprintf("Failed to invoke %s:Prepare()", p.name), err)
		BazelLog.Error(errStr)
		fmt.Print(errStr)
		return EmptyPrepareResult
	}

	// Allow no-return
	if v == starlark.None {
		return EmptyPrepareResult
	}

	BazelLog.Debugf("Invoked plugin %s:prepare(%q): %v\n", p.name, ctx.Rel, v)

	pr, isPR := v.(plugin.PrepareResult)
	if !isPR {
		errStr := fmt.Sprintf("Prepare %v is not a PrepareResult", v)
		BazelLog.Error(errStr)
		fmt.Print(errStr)
		return EmptyPrepareResult
	}

	return pr
}

// Analyze implements plugin.Plugin.
func (p starzellePluginProxy) Analyze(ctx plugin.AnalyzeContext) error {
	if p.analyze == nil {
		return nil
	}
	_, err := starlark.Call(p.t, p.analyze, starlark.Tuple{&ctx}, starUtils.EmptyKwArgs)
	if err != nil {
		errStr := starUtils.ErrorStr(fmt.Sprintf("Failed to invoke %s:Analyze()", p.name), err)
		BazelLog.Error(errStr)
		fmt.Print(errStr)
		return nil
	}
	return nil
}

func (p starzellePluginProxy) DeclareTargets(ctx plugin.DeclareTargetsContext) plugin.DeclareTargetsResult {
	if p.declare == nil {
		return EmptyDeclareTargetsResult
	}

	_, err := starlark.Call(p.t, p.declare, starlark.Tuple{ctx}, starUtils.EmptyKwArgs)
	if err != nil {
		errStr := starUtils.ErrorStr(fmt.Sprintf("Failed to invoke %s:DeclareTargets()", p.name), err)
		BazelLog.Error(errStr)
		fmt.Print(errStr)
		return EmptyDeclareTargetsResult
	}

	actions := ctx.Targets.Actions()

	BazelLog.Debugf("Invoked plugin %s:DeclareTargets(%q): %v\n", p.name, ctx.Rel, actions)
	return plugin.DeclareTargetsResult{
		Actions: actions,
	}
}

func readRuleKind(n starlark.String, v starlark.Value) (plugin.RuleKind, error) {
	from, err1 := starUtils.ReadMapEntry(v, "From", starUtils.ReadString, "")
	matchAny, err2 := starUtils.ReadMapEntry(v, "MatchAny", starUtils.ReadBool, false)
	matchAttrs, err3 := starUtils.ReadMapEntry(v, "MatchAttrs", starUtils.ReadStringList, starUtils.EmptyStrings)
	nonEmptyAttrs, err4 := starUtils.ReadMapEntry(v, "NonEmptyAttrs", starUtils.ReadStringList, starUtils.EmptyStrings)
	mergeableAttrs, err5 := starUtils.ReadMapEntry(v, "MergeableAttrs", starUtils.ReadStringList, starUtils.EmptyStrings)
	resolveAttrs, err6 := starUtils.ReadMapEntry(v, "ResolveAttrs", starUtils.ReadStringList, starUtils.EmptyStrings)

	err := errors.Join(err1, err2, err3, err4, err5, err6)

	return plugin.RuleKind{
		Name: n.GoString(),
		From: from,
		KindInfo: plugin.KindInfo{
			MatchAny:       matchAny,
			MatchAttrs:     matchAttrs,
			NonEmptyAttrs:  nonEmptyAttrs,
			MergeableAttrs: mergeableAttrs,
			ResolveAttrs:   resolveAttrs,
		},
	}, err
}

func readProperty(k string, v starlark.Value) (plugin.Property, error) {
	p, isProp := v.(plugin.Property)

	if !isProp {
		return plugin.Property{}, fmt.Errorf("property %s value %v is not a Property", k, v)
	}

	if p.Name != "" && p.Name != k {
		BazelLog.Errorf("Property name %q does not match key %q", p.Name, k)
	}

	p.Name = k
	return p, nil
}
