package i2vnc

import (
	"fmt"
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
		name         string
		args         args
		wantResolved []string
	}{
		{
			name: "key to key",
			args: args{
				combination: []string{"a"},
				configMaps:  []configMap{{[]string{"a"}, []string{"b"}}},
			},
			wantResolved: []string{"b"},
		},
		{
			name: "mod to mod",
			args: args{
				combination: []string{"Alt_L"},
				configMaps:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
			},
			wantResolved: []string{"Super_L"},
		},
		{
			name: "mod key to key",
			args: args{
				combination: []string{"a", "Alt_L"},
				configMaps:  []configMap{{[]string{"Alt_L", "a"}, []string{"b"}}},
			},
			wantResolved: []string{"b"},
		},
		{
			name: "mod key to mod",
			args: args{
				combination: []string{"a", "Alt_L"},
				configMaps:  []configMap{{[]string{"Alt_L", "a"}, []string{"Super_L"}}},
			},
			wantResolved: []string{"Super_L"},
		},
		{
			name: "mod key to mod key: both multi map",
			args: args{
				combination: []string{"a", "Alt_L"},
				configMaps: []configMap{
					{[]string{"Alt_L", "a"}, []string{"Super_L", "b"}},
					{[]string{"Alt_L"}, []string{"Meta_L"}},
				},
			},
			wantResolved: []string{"Super_L", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResolved := resolve(tt.args.combination, tt.args.configMaps)
			if !reflect.DeepEqual(gotResolved, tt.wantResolved) {
				t.Errorf("resolveMapping() gotResolved = %v, want %v", gotResolved, tt.wantResolved)
			}
		})
	}
}

func Test_event_resolve(t *testing.T) {
	type args struct {
		defs []EventDef
		cms  []configMap
	}
	tests := []struct {
		name         string
		args         args
		wantCurrent  EventDef
		wantResolved []EventDef
	}{
		{
			name: "motion",
			args: args{
				defs: []EventDef{makeEd("Motion", true)},
			},
			wantCurrent:  makeEd("Motion", true),
			wantResolved: []EventDef{makeEd("Motion", true)},
		},
		{
			name: "key press",
			args: args{
				defs: []EventDef{makeEd("a", true)},
			},
			wantCurrent:  makeEd("a", true),
			wantResolved: []EventDef{makeEd("a", true)},
		},
		{
			name: "mod press",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true)},
			},
			wantCurrent:  makeEd("Alt_L", true),
			wantResolved: []EventDef{makeEd("Alt_L", true)},
		},
		{
			name: "mod release",
			args: args{
				defs: []EventDef{makeEd("Alt_L", false)},
			},
			wantCurrent:  makeEd("Alt_L", false),
			wantResolved: []EventDef{makeEd("Alt_L", false)},
		},
		{
			name: "mod press release",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("Alt_L", false)},
			},
			wantCurrent:  makeEd("Alt_L", false),
			wantResolved: []EventDef{makeEd("Alt_L", false)},
		},
		{
			name: "mod resolve press",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true)},
				cms:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
			},
			wantCurrent:  makeEd("Alt_L", true),
			wantResolved: []EventDef{makeEd("Super_L", true)},
		},
		{
			name: "mod key 1",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("a", true)},
				cms:  []configMap{{[]string{"Control_L"}, []string{"Super_L"}}},
			},
			wantCurrent:  makeEd("a", true),
			wantResolved: []EventDef{makeEd("a", true)},
		},
		{
			name: "mod key 2",
			args: args{
				defs: []EventDef{
					makeEd("Alt_L", true), makeEd("a", true),
					makeEd("a", false), makeEd("Alt_L", false),
				},
			},
			wantCurrent:  makeEd("Alt_L", false),
			wantResolved: []EventDef{makeEd("Alt_L", false)},
		},
		{
			name: "mod key resolve press",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("a", true)},
				cms:  []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
			},
			wantCurrent:  makeEd("a", true),
			wantResolved: []EventDef{makeEd("a", true)},
		},
		{
			name: "mod key resolve release",
			args: args{
				defs: []EventDef{
					makeEd("Alt_L", true), makeEd("a", true),
					makeEd("Alt_L", false), makeEd("a", false),
				},
				cms: []configMap{{[]string{"Alt_L"}, []string{"Super_L"}}},
			},
			wantCurrent:  makeEd("a", false),
			wantResolved: []EventDef{makeEd("a", false)},
		},
		{
			name: "key press release",
			args: args{
				defs: []EventDef{makeEd("a", true), makeEd("a", false)},
			},
			wantCurrent:  makeEd("a", false),
			wantResolved: []EventDef{makeEd("a", false)},
		},
		{
			name: "complex step 1",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true)},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Meta_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantCurrent:  makeEd("Alt_L", true),
			wantResolved: []EventDef{makeEd("Meta_L", true)},
		},
		{
			name: "complex step 2",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("Tab", true)},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Meta_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantCurrent:  makeEd("Tab", true),
			wantResolved: []EventDef{makeEd("Tab", true)},
		},
		{
			name: "complex step 3",
			args: args{
				defs: []EventDef{makeEd("Alt_L", true), makeEd("Tab", true), makeEd("Tab", false), makeEd("Tab", true)},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Meta_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantCurrent:  makeEd("Tab", true),
			wantResolved: []EventDef{makeEd("Tab", true)},
		},
		{
			name: "complex step 4",
			args: args{
				defs: []EventDef{
					makeEd("Alt_L", true), makeEd("Tab", true), makeEd("Tab", false),
				},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Meta_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantCurrent:  makeEd("Tab", false),
			wantResolved: []EventDef{makeEd("Tab", false)},
		},
		{
			name: "complex step 5",
			args: args{
				defs: []EventDef{
					makeEd("Alt_L", true), makeEd("Tab", true), makeEd("Tab", false), makeEd("Alt_L", false),
				},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Meta_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantCurrent:  makeEd("Alt_L", false),
			wantResolved: []EventDef{makeEd("Meta_L", false)},
		},
		{
			name: "complex step 6",
			args: args{
				defs: []EventDef{
					makeEd("Alt_L", true), makeEd("Tab", true), makeEd("Tab", false), makeEd("Alt_L", false), makeEd("Motion", false),
				},
				cms: []configMap{
					{[]string{"Alt_L"}, []string{"Meta_L"}},
					{[]string{"Meta_L", "Tab"}, []string{"Super_L", "Tab"}},
				},
			},
			wantCurrent:  makeEd("Motion", false),
			wantResolved: []EventDef{makeEd("Motion", false)},
		},
	}

	for _, tt := range tests {
		e := newEvent(tt.args.cms, 1)
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "complex step 6" {
				fmt.Println("")
			}
			var resolved []EventDef
			for _, def := range tt.args.defs {
				e.handle(def)
				resolved = e.resolve()
			}
			if !reflect.DeepEqual(e.current, tt.wantCurrent) {
				t.Errorf("event.handle() gotDef = %v, want %v", e.current, tt.wantCurrent)
			}
			if !reflect.DeepEqual(resolved, tt.wantResolved) {
				t.Errorf("event.handle() gotResolved = %v, want %v", resolved, tt.wantResolved)
			}
		})
	}
}
