// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package module

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers for capability providers with different cardinalities
// ---------------------------------------------------------------------------

// optionalCapProvider provides an OptionalOne capability.
type optionalCapProvider struct {
	name  string
	cap   string
	value fmt.Stringer
}

func (m optionalCapProvider) Name() string { return m.name }
func (m optionalCapProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        m.cap,
			Cardinality: OptionalOne,
			Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
		},
		Value: m.value,
	}}
}

// manyCapProvider provides a Many capability.
type manyCapProvider struct {
	name  string
	cap   string
	value fmt.Stringer
}

func (m manyCapProvider) Name() string { return m.name }
func (m manyCapProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        m.cap,
			Cardinality: Many,
			Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
		},
		Value: m.value,
	}}
}

// orderedCapProvider provides an OrderedMany capability.
type orderedCapProvider struct {
	name  string
	cap   string
	prov  string
	value fmt.Stringer
}

func (m orderedCapProvider) Name() string { return m.name }
func (m orderedCapProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        m.cap,
			Cardinality: OrderedMany,
			Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
		},
		Name:  m.prov,
		Value: m.value,
	}}
}

// namedCapProvider provides a NamedOne capability.
type namedCapProvider struct {
	name  string
	cap   string
	prov  string
	value fmt.Stringer
}

func (m namedCapProvider) Name() string { return m.name }
func (m namedCapProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        m.cap,
			Cardinality: NamedOne,
			Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
		},
		Name:  m.prov,
		Value: m.value,
	}}
}

// ---------------------------------------------------------------------------
// ResolveOptionalOne
// ---------------------------------------------------------------------------

func TestResolveOptionalOne_Present(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(optionalCapProvider{
		name:  "p1",
		cap:   "opt.cap",
		value: namedStringer("hello"),
	}))
	require.NoError(t, h.Seal())

	got, ok, err := ResolveOptionalOne[fmt.Stringer](h, CapabilitySpec{
		Name:        "opt.cap",
		Cardinality: OptionalOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "hello", got.String())
}

func TestResolveOptionalOne_Absent(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))
	require.NoError(t, h.Seal())

	got, ok, err := ResolveOptionalOne[fmt.Stringer](h, CapabilitySpec{
		Name:        "opt.cap",
		Cardinality: OptionalOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, got)
}

func TestResolveOptionalOne_TooMany(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		optionalCapProvider{name: "p1", cap: "opt.cap", value: namedStringer("a")},
		optionalCapProvider{name: "p2", cap: "opt.cap", value: namedStringer("b")},
	))
	err := h.Seal()
	require.Error(t, err)
	require.Contains(t, err.Error(), "at most one provider")
}

func TestResolveOptionalOne_WrongCardinality(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))
	require.NoError(t, h.Seal())

	_, _, err := ResolveOptionalOne[fmt.Stringer](h, CapabilitySpec{
		Name:        "opt.cap",
		Cardinality: ExactlyOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires optional_one cardinality")
}

func TestResolveOptionalOne_UnsealedHub(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))

	_, _, err := ResolveOptionalOne[fmt.Stringer](h, CapabilitySpec{
		Name:        "opt.cap",
		Cardinality: OptionalOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.Error(t, err)
	require.Equal(t, errHubNotSealed, err)
}

// ---------------------------------------------------------------------------
// ResolveMany
// ---------------------------------------------------------------------------

func TestResolveMany_MultipleProviders(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		manyCapProvider{name: "p1", cap: "many.cap", value: namedStringer("alpha")},
		manyCapProvider{name: "p2", cap: "many.cap", value: namedStringer("beta")},
		manyCapProvider{name: "p3", cap: "many.cap", value: namedStringer("gamma")},
	))
	require.NoError(t, h.Seal())

	got, err := ResolveMany[fmt.Stringer](h, CapabilitySpec{
		Name:        "many.cap",
		Cardinality: Many,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.NoError(t, err)
	require.Len(t, got, 3)
	vals := make([]string, len(got))
	for i, v := range got {
		vals[i] = v.String()
	}
	require.Contains(t, vals, "alpha")
	require.Contains(t, vals, "beta")
	require.Contains(t, vals, "gamma")
}

func TestResolveMany_Empty(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))
	require.NoError(t, h.Seal())

	got, err := ResolveMany[fmt.Stringer](h, CapabilitySpec{
		Name:        "many.cap",
		Cardinality: Many,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestResolveMany_WrongCardinality(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))
	require.NoError(t, h.Seal())

	_, err := ResolveMany[fmt.Stringer](h, CapabilitySpec{
		Name:        "many.cap",
		Cardinality: ExactlyOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires many cardinality")
}

func TestResolveMany_UnsealedHub(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))

	_, err := ResolveMany[fmt.Stringer](h, CapabilitySpec{
		Name:        "many.cap",
		Cardinality: Many,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	})
	require.Error(t, err)
	require.Equal(t, errHubNotSealed, err)
}

// ---------------------------------------------------------------------------
// ResolveOrdered
// ---------------------------------------------------------------------------

func TestResolveOrdered_Nominal(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		orderedCapProvider{name: "m1", cap: "ord.cap", prov: "first", value: namedStringer("A")},
		orderedCapProvider{name: "m2", cap: "ord.cap", prov: "second", value: namedStringer("B")},
		orderedCapProvider{name: "m3", cap: "ord.cap", prov: "third", value: namedStringer("C")},
	))
	require.NoError(t, h.Seal())

	got, err := ResolveOrdered[fmt.Stringer](h, CapabilitySpec{
		Name:        "ord.cap",
		Cardinality: OrderedMany,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, []string{"second", "first", "third"})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, "B", got[0].String())
	require.Equal(t, "A", got[1].String())
	require.Equal(t, "C", got[2].String())
}

func TestResolveOrdered_DuplicateName(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		orderedCapProvider{name: "m1", cap: "ord.cap", prov: "first", value: namedStringer("A")},
	))
	require.NoError(t, h.Seal())

	_, err := ResolveOrdered[fmt.Stringer](h, CapabilitySpec{
		Name:        "ord.cap",
		Cardinality: OrderedMany,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, []string{"first", "first"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicated in ordered list")
}

func TestResolveOrdered_MissingProvider(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		orderedCapProvider{name: "m1", cap: "ord.cap", prov: "first", value: namedStringer("A")},
	))
	require.NoError(t, h.Seal())

	_, err := ResolveOrdered[fmt.Stringer](h, CapabilitySpec{
		Name:        "ord.cap",
		Cardinality: OrderedMany,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, []string{"nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestResolveOrdered_WrongCardinality(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))
	require.NoError(t, h.Seal())

	_, err := ResolveOrdered[fmt.Stringer](h, CapabilitySpec{
		Name:        "ord.cap",
		Cardinality: Many,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires ordered_many cardinality")
}

// ---------------------------------------------------------------------------
// ResolveNamed
// ---------------------------------------------------------------------------

func TestResolveNamed_Found(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(namedCapProvider{
		name:  "m1",
		cap:   "named.cap",
		prov:  "alpha",
		value: namedStringer("value-a"),
	}))
	require.NoError(t, h.Seal())

	got, err := ResolveNamed[fmt.Stringer](h, CapabilitySpec{
		Name:        "named.cap",
		Cardinality: NamedOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, "alpha")
	require.NoError(t, err)
	require.Equal(t, "value-a", got.String())
}

func TestResolveNamed_NotFound(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(namedCapProvider{
		name:  "m1",
		cap:   "named.cap",
		prov:  "alpha",
		value: namedStringer("value-a"),
	}))
	require.NoError(t, h.Seal())

	_, err := ResolveNamed[fmt.Stringer](h, CapabilitySpec{
		Name:        "named.cap",
		Cardinality: NamedOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, "nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestResolveNamed_DuplicateNameDetected(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(
		namedCapProvider{name: "m1", cap: "named.cap", prov: "alpha", value: namedStringer("a")},
		namedCapProvider{name: "m2", cap: "named.cap", prov: "alpha", value: namedStringer("b")},
	))
	err := h.Seal()
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate provider name")
}

func TestResolveNamed_UnsealedHub(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))

	_, err := ResolveNamed[fmt.Stringer](h, CapabilitySpec{
		Name:        "named.cap",
		Cardinality: NamedOne,
		Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
	}, "alpha")
	require.Error(t, err)
	require.Equal(t, errHubNotSealed, err)
}

// ---------------------------------------------------------------------------
// Helper functions: slicesUniq, normalizeModuleConflicts, modulesOfEntries
// ---------------------------------------------------------------------------

func TestSlicesUniq_Empty(t *testing.T) {
	result := slicesUniq(nil)
	require.Nil(t, result)
}

func TestSlicesUniq_DeduplicatesAndSorts(t *testing.T) {
	result := slicesUniq([]string{"c", "a", "b", "a", "c"})
	require.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSlicesUniq_FiltersEmptyStrings(t *testing.T) {
	result := slicesUniq([]string{"", "a", "", "b", ""})
	require.Equal(t, []string{"a", "b"}, result)
}

func TestSlicesUniq_AllDuplicates(t *testing.T) {
	result := slicesUniq([]string{"x", "x", "x"})
	require.Equal(t, []string{"x"}, result)
}

func TestNormalizeModuleConflicts_Empty(t *testing.T) {
	result := normalizeModuleConflicts(nil)
	require.Empty(t, result)
}

func TestNormalizeModuleConflicts_Deduplicates(t *testing.T) {
	result := normalizeModuleConflicts(map[string][]string{
		"m1": {"err1", "err2", "err1"},
	})
	require.Equal(t, map[string][]string{
		"m1": {"err1", "err2"},
	}, result)
}

func TestModulesOfEntries_Deduplicates(t *testing.T) {
	m1 := &testModule{name: "a"}
	m2 := &testModule{name: "b"}
	entries := []capabilityEntry{
		{module: m1},
		{module: m1},
		{module: m2},
	}
	result := modulesOfEntries(entries)
	require.Len(t, result, 2)
	require.Equal(t, "a", result[0].Name())
	require.Equal(t, "b", result[1].Name())
}

func TestModulesOfEntries_Empty(t *testing.T) {
	result := modulesOfEntries(nil)
	require.Empty(t, result)
}

// ---------------------------------------------------------------------------
// CapabilityCardinality.String
// ---------------------------------------------------------------------------

func TestCapabilityCardinality_String(t *testing.T) {
	tests := []struct {
		input    CapabilityCardinality
		expected string
	}{
		{ExactlyOne, "exactly_one"},
		{OptionalOne, "optional_one"},
		{Many, "many"},
		{OrderedMany, "ordered_many"},
		{NamedOne, "named_one"},
		{CapabilityCardinality(99), "unknown"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, tt.input.String())
	}
}

// ---------------------------------------------------------------------------
// typeMatches
// ---------------------------------------------------------------------------

func TestTypeMatches_NilExpect(t *testing.T) {
	require.True(t, typeMatches(nil, reflect.TypeOf("hello")))
}

func TestTypeMatches_NilActual(t *testing.T) {
	require.False(t, typeMatches(reflect.TypeOf(0), nil))
}

func TestTypeMatches_InterfaceMatch(t *testing.T) {
	stringerType := reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	actual := reflect.TypeOf(namedStringer(""))
	require.True(t, typeMatches(stringerType, actual))
}

func TestTypeMatches_AssignableTo(t *testing.T) {
	expect := reflect.TypeOf(int(0))
	actual := reflect.TypeOf(int(0))
	require.True(t, typeMatches(expect, actual))
}

func TestTypeMatches_NotAssignable(t *testing.T) {
	expect := reflect.TypeOf(int(0))
	actual := reflect.TypeOf("string")
	require.False(t, typeMatches(expect, actual))
}

// ---------------------------------------------------------------------------
// castValue
// ---------------------------------------------------------------------------

func TestCastValue_Success(t *testing.T) {
	got, err := castValue[fmt.Stringer](namedStringer("test"))
	require.NoError(t, err)
	require.Equal(t, "test", got.String())
}

func TestCastValue_TypeMismatch(t *testing.T) {
	_, err := castValue[fmt.Stringer](42)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type mismatch")
}

// ---------------------------------------------------------------------------
// capabilityEntries edge case: empty spec name
// ---------------------------------------------------------------------------

func TestCapabilityEntries_EmptySpecName(t *testing.T) {
	h := NewHub()
	require.NoError(t, h.Use(&testModule{name: "m1"}))
	require.NoError(t, h.Seal())

	_, err := h.capabilityEntries(CapabilitySpec{Name: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec name is required")
}

// ---------------------------------------------------------------------------
// collectCapabilities edge cases
// ---------------------------------------------------------------------------

func TestCollectCapabilities_EmptySpecName(t *testing.T) {
	p := &emptySpecProvider{}
	_, err := collectCapabilities([]Module{p})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty spec name")
}

func TestCollectCapabilities_NilValue(t *testing.T) {
	p := &nilValueProvider{}
	_, err := collectCapabilities([]Module{p})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil value")
}

func TestCollectCapabilities_CardinalityMismatch(t *testing.T) {
	p1 := &mismatchCardProvider{card: ExactlyOne}
	p2 := &mismatchCardProvider{card: OptionalOne}
	_, err := collectCapabilities([]Module{p1, p2})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cardinality mismatch")
}

func TestCollectCapabilities_SpecTypeMismatch(t *testing.T) {
	// Both providers declare Many cardinality with matching interface type
	// but different concrete Spec.Type values (both non-nil)
	p1 := &specTypeProvider{typ: reflect.TypeOf(int(0)), val: int(42)}
	p2 := &specTypeProvider{typ: reflect.TypeOf("str"), val: "hello"}
	_, err := collectCapabilities([]Module{p1, p2})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type mismatch")
}

func TestCollectCapabilities_ValueTypeMismatch(t *testing.T) {
	p := &valueTypeMismatchProvider{}
	_, err := collectCapabilities([]Module{p})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match")
}

func TestCollectCapabilities_ExactlyOneViolation(t *testing.T) {
	p1 := capProvider{name: "p1"}
	p2 := capProvider{name: "p2"}
	_, err := collectCapabilities([]Module{p1, p2})
	require.Error(t, err)
	require.Contains(t, err.Error(), "got 2")
}

func TestCollectCapabilities_OptionalOneViolation(t *testing.T) {
	p1 := optionalCapProvider{name: "p1", cap: "opt.cap", value: namedStringer("a")}
	p2 := optionalCapProvider{name: "p2", cap: "opt.cap", value: namedStringer("b")}
	_, err := collectCapabilities([]Module{p1, p2})
	require.Error(t, err)
	require.Contains(t, err.Error(), "at most one provider")
}

func TestCollectCapabilities_NilTypeInferredFromValue(t *testing.T) {
	p := &nilTypeProvider{}
	result, err := collectCapabilities([]Module{p})
	require.NoError(t, err)
	entries := result.index["infer.cap"]
	require.Len(t, entries, 1)
	require.Equal(t, reflect.TypeOf(namedStringer("")), entries[0].spec.Type)
}

func TestCollectCapabilities_NilTypeInheritsFromPrevious(t *testing.T) {
	p1 := &nilTypeProvider{}       // first provider, Type=nil, value is namedStringer
	p2 := &nilTypeSecondProvider{} // second provider, Type=nil, value is namedStringer
	result, err := collectCapabilities([]Module{p1, p2})
	require.NoError(t, err)
	entries := result.index["infer.cap"]
	require.Len(t, entries, 2)
}

// ---------------------------------------------------------------------------
// Providers used by collectCapabilities edge-case tests
// ---------------------------------------------------------------------------

type emptySpecProvider struct{}

func (m *emptySpecProvider) Name() string { return "empty-spec" }
func (m *emptySpecProvider) Capabilities() []Capability {
	return []Capability{{
		Spec:  CapabilitySpec{Name: "", Cardinality: ExactlyOne},
		Value: namedStringer("x"),
	}}
}

type nilValueProvider struct{}

func (m *nilValueProvider) Name() string { return "nil-val" }
func (m *nilValueProvider) Capabilities() []Capability {
	return []Capability{{
		Spec:  CapabilitySpec{Name: "nil.cap", Cardinality: ExactlyOne},
		Value: nil,
	}}
}

type mismatchCardProvider struct{ card CapabilityCardinality }

func (m *mismatchCardProvider) Name() string { return "mismatch-" + m.card.String() }
func (m *mismatchCardProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        "same.cap",
			Cardinality: m.card,
			Type:        reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
		},
		Value: namedStringer("v"),
	}}
}

// specTypeProvider uses a concrete value matching its Spec.Type
// so we can test the Spec.Type comparison between two providers.
type specTypeProvider struct {
	typ reflect.Type
	val any
}

func (m *specTypeProvider) Name() string { return "spec-type-" + m.typ.String() }
func (m *specTypeProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        "same.cap",
			Cardinality: Many,
			Type:        m.typ,
		},
		Value: m.val,
	}}
}

type valueTypeMismatchProvider struct{}

func (m *valueTypeMismatchProvider) Name() string { return "val-type" }
func (m *valueTypeMismatchProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        "bad.cap",
			Cardinality: ExactlyOne,
			Type:        reflect.TypeOf(0),
		},
		Value: namedStringer("not-an-int"),
	}}
}

type nilTypeProvider struct{}

func (m *nilTypeProvider) Name() string { return "nil-type" }
func (m *nilTypeProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        "infer.cap",
			Cardinality: Many,
			Type:        nil,
		},
		Value: namedStringer("v"),
	}}
}

type nilTypeSecondProvider struct{}

func (m *nilTypeSecondProvider) Name() string { return "nil-type-2" }
func (m *nilTypeSecondProvider) Capabilities() []Capability {
	return []Capability{{
		Spec: CapabilitySpec{
			Name:        "infer.cap",
			Cardinality: Many,
			Type:        nil,
		},
		Value: namedStringer("v2"),
	}}
}
