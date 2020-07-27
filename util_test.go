package i2vnc

import (
	"reflect"
	"testing"
)

func makeEd(name string, isPress bool) EventDef {
	ed, _ := newEventDefByName(name, isPress)
	return *ed
}

func Test_stringSliceEquals(t *testing.T) {
	type args struct {
		a []string
		b []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"success identical", args{[]string{"a", "b"}, []string{"a", "b"}}, true},
		{"success shifted", args{[]string{"a", "b"}, []string{"b", "a"}}, true},
		{"fail empty", args{[]string{"a", "b"}, []string{}}, false},
		{"fail different elem", args{[]string{"a", "b"}, []string{"a", "c"}}, false},
		{"fail comparison smaller", args{[]string{"a", "b"}, []string{"a"}}, false},
		{"fail comparison bigger", args{[]string{"a", "b"}, []string{"a", "b", "c"}}, false},
		{"fail source smaller", args{[]string{"a"}, []string{"a", "b"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringSliceEquals(tt.args.a, tt.args.b); got != tt.want {
				t.Errorf("stringSliceEquals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resolveMapping(t *testing.T) {
	type args struct {
		combination []string
		configMaps  []configMap
		isPress     bool
	}
	tests := []struct {
		name              string
		args              args
		wantResolved      []EventDef
		wantSkipOnRelease []string
	}{
		{
			name: "key to key",
			args: args{
				combination: []string{"a"},
				configMaps:  []configMap{{[]string{"a"}, []string{"b"}}},
				isPress:     true,
			},
			wantResolved:      []EventDef{makeEd("b", true)},
			wantSkipOnRelease: nil,
		},
		{
			name: "mod to mod",
			args: args{
				combination: []string{"Alt_L"},
				configMaps:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
				isPress:     true,
			},
			wantResolved:      []EventDef{makeEd("Super_L", true)},
			wantSkipOnRelease: nil,
		},
		{
			name: "mod key to key",
			args: args{
				combination: []string{"a", "Alt_L"},
				configMaps:  []configMap{{[]string{"Alt_L", "a"}, []string{"b"}}},
				isPress:     true,
			},
			wantResolved:      []EventDef{makeEd("Alt_L", false), makeEd("b", true)},
			wantSkipOnRelease: []string{"Alt_L"},
		},
		{
			name: "mod key to mod",
			args: args{
				combination: []string{"a", "Alt_L"},
				configMaps:  []configMap{{[]string{"Alt_L", "a"}, []string{"Super_L"}}},
				isPress:     true,
			},
			wantResolved:      []EventDef{makeEd("Alt_L", false), makeEd("Super_L", true)},
			wantSkipOnRelease: []string{"Alt_L"},
		},
		{
			name: "mod key to mod key: mod",
			args: args{
				combination: []string{"Alt_L", "a"},
				configMaps:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
				isPress:     true,
			},
			wantResolved:      []EventDef{makeEd("Alt_L", true), makeEd("a", true)},
			wantSkipOnRelease: nil,
		},
		{
			name: "mod key to mod key: both multi map",
			args: args{
				combination: []string{"a", "Alt_L"},
				configMaps: []configMap{
					{[]string{"Alt_L", "a"}, []string{"Super_L", "b"}},
					{[]string{"Alt_L"}, []string{"Meta_L"}},
				},
				isPress: true,
			},
			wantResolved:      []EventDef{makeEd("Meta_L", false), makeEd("Super_L", true), makeEd("b", true)},
			wantSkipOnRelease: []string{"Meta_L"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResolved, gotSkipOnRelease := resolveCombination(tt.args.combination, tt.args.configMaps, tt.args.isPress)
			if !reflect.DeepEqual(gotResolved, tt.wantResolved) {
				t.Errorf("resolveMapping() gotResolved = %v, want %v", gotResolved, tt.wantResolved)
			}
			if !reflect.DeepEqual(gotSkipOnRelease, tt.wantSkipOnRelease) {
				t.Errorf("resolveMapping() gotSkipOnRelease = %v, want %v", gotSkipOnRelease, tt.wantSkipOnRelease)
			}
		})
	}
}

func Test_event_handle(t *testing.T) {
	type args struct {
		defs []EventDef
		cms  []configMap
	}
	tests := []struct {
		name         string
		args         args
		wantDef      EventDef
		wantMods     map[string]string
		wantResolved []EventDef
		wantSkip     []string
	}{
		{
			name: "key press",
			args: args{
				defs: []EventDef{makeEd("a", true)},
			},
			wantDef:      makeEd("a", true),
			wantMods:     map[string]string{},
			wantResolved: []EventDef{makeEd("a", true)},
		},
		{
			name: "mod resolve press",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true)},
				cms:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
			},
			wantDef:      makeEd("Alt_L", true),
			wantMods:     map[string]string{"Alt_L": "Alt_L"},
			wantResolved: []EventDef{makeEd("Super_L", true)},
		},
		{
			name: "mod key press",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("a", true)},
			},
			wantDef:      makeEd("a", true),
			wantMods:     map[string]string{"Alt_L": "Alt_L"},
			wantResolved: []EventDef{makeEd("a", true)},
		},
		{
			name: "mod key release",
			args: args{
				defs: []EventDef{makeEd("Alt_L", false), makeEd("a", false)},
			},
			wantDef:      makeEd("a", false),
			wantMods:     map[string]string{},
			wantResolved: []EventDef{makeEd("a", false)},
		},
		{
			name: "mod key resolve press",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("a", true)},
				cms:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
			},
			wantDef:      makeEd("a", true),
			wantMods:     map[string]string{"Alt_L": "Alt_L"},
			wantResolved: []EventDef{makeEd("a", true)},
		},
		{
			name: "mod key resolve release",
			args: args{
				defs: []EventDef{makeEd("Alt_L", false), makeEd("a", false)},
				cms:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
			},
			wantDef:      makeEd("a", false),
			wantMods:     map[string]string{},
			wantResolved: []EventDef{makeEd("a", false)},
		},
		{
			name: "key press release",
			args: args{
				defs: []EventDef{makeEd("a", true), makeEd("a", false)},
			},
			wantDef:      makeEd("a", false),
			wantMods:     map[string]string{},
			wantResolved: []EventDef{makeEd("a", false)},
		},
		{
			name: "mod key resolve press example",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("Tab", true)},
				cms:  []configMap{{[]string{"Alt_L"}, []string{"Meta_L"}}},
			},
			wantDef:      makeEd("Tab", true),
			wantMods:     map[string]string{"Alt_L": "Alt_L"},
			wantResolved: []EventDef{makeEd("Tab", true)},
		},
		{
			name: "complex step 1",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true)},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Alt_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantDef:      makeEd("Alt_L", true),
			wantMods:     map[string]string{"Alt_L": "Alt_L"},
			wantResolved: []EventDef{makeEd("Meta_L", true)},
		},
		{
			name: "complex step 2",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("Tab", true)},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Alt_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantDef:      makeEd("Tab", true),
			wantMods:     map[string]string{"Alt_L": "Alt_L"},
			wantResolved: []EventDef{makeEd("Meta_L", false), makeEd("Super_L", true), makeEd("Tab", true)},
			wantSkip:     []string{"Meta_L"},
		},
		{
			name: "complex step 3",
			args: args{
				defs: []EventDef{
					makeEd("Alt_L", true), makeEd("Tab", true), makeEd("Tab", false),
				},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Alt_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantDef:      makeEd("Tab", false),
			wantMods:     map[string]string{"Alt_L": "Alt_L"},
			wantResolved: []EventDef{makeEd("Super_L", false), makeEd("Tab", false)},
			wantSkip:     []string{"Meta_L"},
		},
		{
			name: "complex step 4",
			args: args{
				defs: []EventDef{
					makeEd("Alt_L", true), makeEd("Tab", true), makeEd("Tab", false), makeEd("Alt_L", false),
				},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Alt_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantDef:      makeEd("Alt_L", false),
			wantMods:     map[string]string{},
			wantResolved: nil,
			wantSkip:     []string{},
		},
	}
	e := newEvent(nil, 1)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, def := range tt.args.defs {
				e.handle(def)
			}
			resolved := e.resolve(tt.args.cms, 1)
			if !reflect.DeepEqual(e.def, tt.wantDef) {
				t.Errorf("event.handle() gotDef = %v, want %v", e.def, tt.wantDef)
			}
			if !reflect.DeepEqual(e.modMap, tt.wantMods) {
				t.Errorf("event.handle() gotMods = %v, want %v", e.modMap, tt.wantMods)
			}
			if !reflect.DeepEqual(resolved, tt.wantResolved) {
				t.Errorf("event.handle() gotResolved = %v, want %v", resolved, tt.wantResolved)
			}
			if !reflect.DeepEqual(e.skipOnRelease, tt.wantSkip) {
				t.Errorf("event.handle() gotSkip = %v, want %v", e.skipOnRelease, tt.wantSkip)
			}
		})
	}
}
