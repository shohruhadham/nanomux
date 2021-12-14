// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"reflect"
	"regexp"
	"testing"
)

func TestSimilarity_Err(t *testing.T) {
	var cases = []struct {
		name    string
		s       Similarity
		wantErr error
	}{
		{"Different", Different, ErrDifferentTemplates},
		{"DifferentValueNames", DifferentValueNames, ErrDifferentValueNames},
		{"DifferentNames", DifferentNames, ErrDifferentNames},
		{"TheSame", TheSame, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.s.Err(); err != c.wantErr {
				t.Fatalf(
					"Similarity.Err() error = %v, wantErr %v",
					err,
					c.wantErr,
				)
			}
		})
	}
}

func TestTemplate_SetName(t *testing.T) {
	var tmpl = &Template{}
	var wantTmpl = &Template{name: "name"}

	tmpl.SetName("name")
	if !reflect.DeepEqual(tmpl, wantTmpl) {
		t.Fatalf("after Template.SetName() tmpl = %v, want %v", tmpl, wantTmpl)
	}
}

func TestTemplate_Name(t *testing.T) {
	var wantName = "name"
	var tmpl = &Template{name: wantName}
	if name := tmpl.Name(); name != wantName {
		t.Fatalf("Template.Name() = %v, want %v", name, wantName)
	}
}

func TestTemplate_Content(t *testing.T) {
	var cases = []struct {
		name string
		tmpl *Template
		want string
	}{
		{
			"static",
			&Template{slices: []_TemplateSlice{{staticStr: "static template"}}},
			"static template",
		},
		{
			"pattern",
			&Template{slices: []_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						name: "pattern",
						re:   regexp.MustCompile(`\d{3}`),
					},
				},
			}},
			`{pattern:\d{3}}`,
		},
		{
			"static pattern",
			&Template{slices: []_TemplateSlice{
				{staticStr: "static "},
				{
					valuePattern: &_ValuePattern{
						name: "pattern",
						re:   regexp.MustCompile(`\d{3}`),
					},
				},
			}},
			`static {pattern:\d{3}}`,
		},
		{
			"pattern static",
			&Template{slices: []_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						name: "pattern",
						re:   regexp.MustCompile(`\d{3}`),
					},
				},
				{staticStr: " static "},
			}},
			`{pattern:\d{3}} static `,
		},
		{
			"static pattern static",
			&Template{slices: []_TemplateSlice{
				{staticStr: "static{slice}"},
				{
					valuePattern: &_ValuePattern{
						name: "pattern {slice}",
						re:   regexp.MustCompile(`\d{3}`),
					},
				},
				{staticStr: " {static} slice"},
			}},
			`static\{slice\}{pattern {slice}:\d{3}} \{static\} slice`,
		},
		{
			"pattern static pattern",
			&Template{slices: []_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						name: `pattern:{slice}`,
						re:   regexp.MustCompile(`\d{3}`),
					},
				},
				{staticStr: "{static} slice"},
				{
					valuePattern: &_ValuePattern{
						name: "pattern-2:{slice}",
						re:   regexp.MustCompile(`\d{3}\{\}`),
					},
				},
			}},
			`{pattern\:{slice}:\d{3}}\{static\} slice{pattern-2\:{slice}:\d{3}\{\}}`,
		},
		{
			"pattern static wildcard pattern",
			&Template{slices: []_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						name: `pattern:{1}`,
						re:   regexp.MustCompile(`\d{3}`),
					},
				},
				{staticStr: "{static slice}"},
				{
					valuePattern: &_ValuePattern{name: "wildcard:{slice}"},
				},
				{
					valuePattern: &_ValuePattern{
						name: "pattern:{2}",
						re:   regexp.MustCompile(`\d{3}`),
					},
				},
			}},
			`{pattern\:{1}:\d{3}}\{static slice\}{wildcard\:{slice}}{pattern\:{2}:\d{3}}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tmpl.Content(); got != c.want {
				t.Fatalf("Template.Content() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTemplate_IsStatic(t *testing.T) {
	var cases = []struct {
		name string
		tmpl *Template
		want bool
	}{
		{
			"static template",
			&Template{slices: []_TemplateSlice{{staticStr: "static template"}}},
			true,
		},
		{
			"pattern template",
			&Template{slices: []_TemplateSlice{{
				valuePattern: &_ValuePattern{
					"pattern",
					regexp.MustCompile(`\d{3}`),
				},
			}}},
			false,
		},
		{
			"wildcard template",
			&Template{slices: []_TemplateSlice{{
				valuePattern: &_ValuePattern{name: "wildcard"},
			}}},
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tmpl.IsStatic(); got != c.want {
				t.Fatalf("Template.IsStatic() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTemplate_IsWildCard(t *testing.T) {
	var cases = []struct {
		name string
		tmpl *Template
		want bool
	}{
		{
			"wildcard template",
			&Template{slices: []_TemplateSlice{{
				valuePattern: &_ValuePattern{name: "wildcard"},
			}}},
			true,
		},
		{
			"pattern template",
			&Template{slices: []_TemplateSlice{{
				valuePattern: &_ValuePattern{
					"pattern",
					regexp.MustCompile(`\d{3}`),
				},
			}}},
			false,
		},
		{
			"static template",
			&Template{slices: []_TemplateSlice{{staticStr: "static template"}}},
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tmpl.IsWildcard(); got != c.want {
				t.Fatalf("Template.IsWildCard() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTemplate_SimilarityWith(t *testing.T) {
	var tmpl = &Template{
		name: "tmpl",
		slices: []_TemplateSlice{
			{valuePattern: &_ValuePattern{"id1", regexp.MustCompile(`\d{3}`)}},
			{staticStr: ", "},
			{valuePattern: &_ValuePattern{"id2", regexp.MustCompile(`\d{2}`)}},
			{staticStr: " - IDs; name: "},
			{valuePattern: &_ValuePattern{name: "name"}},
		},
		wildCardIdx: 4,
	}

	var cases = []struct {
		name string
		tmpl *Template
		want Similarity
	}{
		{
			"different #1",
			&Template{name: "tmpl", slices: []_TemplateSlice{
				{staticStr: "green-energy"},
			}},
			Different,
		},
		{
			"different #2",
			&Template{name: "tmpl", slices: []_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`\d{3}`),
					},
				},
				{staticStr: " - IDs"},
			}},
			Different,
		},
		{
			"different #3",
			&Template{
				name: "tmpl",
				slices: []_TemplateSlice{
					{staticStr: "name: "},
					{valuePattern: &_ValuePattern{name: "name"}},
					{valuePattern: &_ValuePattern{
						"id1",
						regexp.MustCompile(`\d{3}`),
					}},
					{staticStr: ", "},
					{valuePattern: &_ValuePattern{
						"id2",
						regexp.MustCompile(`\d{2}`),
					}},
				},
				wildCardIdx: 1,
			},
			Different,
		},
		{
			"different #4",
			&Template{name: "tmpl", slices: []_TemplateSlice{
				{valuePattern: &_ValuePattern{name: "forest name"}},
			}},
			Different,
		},
		{
			"different value names #1",
			&Template{
				name: "tmpl",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{
						"id-1",
						regexp.MustCompile(`\d{3}`),
					}},
					{staticStr: ", "},
					{valuePattern: &_ValuePattern{
						"id-2",
						regexp.MustCompile(`\d{2}`),
					}},
					{staticStr: " - IDs; name: "},
					{valuePattern: &_ValuePattern{name: "name"}},
				},
				wildCardIdx: 4,
			},
			DifferentValueNames,
		},
		{
			"different value names #2",
			&Template{
				name: "tmpl",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{
						"id1",
						regexp.MustCompile(`\d{3}`),
					}},
					{staticStr: ", "},
					{valuePattern: &_ValuePattern{
						"id2",
						regexp.MustCompile(`\d{2}`),
					}},
					{staticStr: " - IDs; name: "},
					{valuePattern: &_ValuePattern{name: "Name"}},
				},
				wildCardIdx: 4,
			},
			DifferentValueNames,
		},
		{
			"different names",
			&Template{
				name: "template",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{
						"id1",
						regexp.MustCompile(`\d{3}`),
					}},
					{staticStr: ", "},
					{valuePattern: &_ValuePattern{
						"id2",
						regexp.MustCompile(`\d{2}`),
					}},
					{staticStr: " - IDs; name: "},
					{valuePattern: &_ValuePattern{name: "name"}},
				},
				wildCardIdx: 4,
			},
			DifferentNames,
		},
		{
			"the same",
			&Template{
				name: "tmpl",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{
						"id1",
						regexp.MustCompile(`\d{3}`),
					}},
					{staticStr: ", "},
					{valuePattern: &_ValuePattern{
						"id2",
						regexp.MustCompile(`\d{2}`),
					}},
					{staticStr: " - IDs; name: "},
					{valuePattern: &_ValuePattern{name: "name"}},
				},
				wildCardIdx: 4,
			},
			TheSame,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tmpl.SimilarityWith(c.tmpl); got != c.want {
				t.Fatalf("Template.SimilarityWith() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTemplate_Match(t *testing.T) {
	var cases = []struct {
		name            string
		tmpl            *Template
		matchingStr     string
		wantValues      map[string]string
		nonMatchingStrs []string
	}{
		{
			"static",
			Parse("green-energy"),
			"green-energy",
			nil,
			[]string{"solar-power"},
		},
		{
			"wildcard",
			Parse("{river}"),
			"Sir-Daryo",
			map[string]string{"river": "Sir-Daryo"},
			nil,
		},
		{
			"pattern",
			Parse("{id:\\d{3}}"),
			"123",
			map[string]string{"id": "123"},
			[]string{"abc", "12", "12345"},
		},
		{
			"pattern pattern",
			Parse("{name:[A-Za-z]+}{id:\\d+}"),
			"abc123",
			map[string]string{"name": "abc", "id": "123"},
			[]string{"abc", "12", "1234", " 123", "123 ", " 123 ", "123ab"},
		},
		{
			"static pattern static pattern",
			Parse("name: {name:[A-Za-z]{3}}, id: {id:\\d{3}}"),
			"name: abc, id: 123",
			map[string]string{"name": "abc", "id": "123"},
			[]string{
				"name: abc", "id: 123", "name: abc, id: 12", "name: 123, id: abc", "id: 123, name: abc",
			},
		},
		{
			"static pattern static wildcard",
			Parse("name: {name:[A-Za-z]{3}}, address: {address}"),
			"name: abc, address: Kepler-452b",
			map[string]string{"name": "abc", "address": "Kepler-452b"},
			[]string{
				"name: abc", "address: Mars", "address: Ocean, name: abc",
			},
		},
		{
			"static pattern static wildcard static pattern",
			Parse(
				"id: {id:\\d{3}}, address: {address}, state: {state:(unknown|active|dormant)}",
			),
			"id: 123, address: Proxima b, state: active",
			map[string]string{
				"id":      "123",
				"address": "Proxima b",
				"state":   "active",
			},
			[]string{
				"id: 321, address: Moon, state: unclear", "address: Mars", "id: 12, address: Ocean, state: dormant",
			},
		},
		{
			"wildcard static pattern",
			Parse("{galaxy}, {color:(red|blue|white)}"),
			"Eye of Sauron, red",
			map[string]string{"galaxy": "Eye of Sauron", "color": "red"},
			[]string{
				"Medusa Merger, yellow", "Malin 1", "white",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var matched, values = c.tmpl.Match(c.matchingStr, nil)
			if !matched {
				t.Fatalf("Template.Match() matched = false, want true")
			}

			if !reflect.DeepEqual(values, c.wantValues) {
				t.Fatalf(
					"Template.Match() values = %v, want %v",
					values,
					c.wantValues,
				)
			}

			for _, str := range c.nonMatchingStrs {
				var matched, _ = c.tmpl.Match(str, nil)
				if matched {
					t.Fatalf("Template.Match() matched = true, want false")
				}
			}
		})
	}
}

func TestTemplate_Apply(t *testing.T) {
	var cases = []struct {
		name          string
		tmpl          *Template
		values        map[string]string
		ignoreMissing bool
		resultStr     string
		wantErr       bool
	}{
		{
			"static",
			&Template{slices: []_TemplateSlice{{staticStr: "green-energy"}}},
			map[string]string{"key": "value"},
			false,
			"green-energy",
			false,
		},
		{
			"wildcard",
			&Template{slices: []_TemplateSlice{
				{valuePattern: &_ValuePattern{name: "river"}},
			}},
			map[string]string{"river": "Sir-Daryo"},
			false,
			"Sir-Daryo",
			false,
		},
		{
			"pattern",
			&Template{slices: []_TemplateSlice{
				{valuePattern: &_ValuePattern{
					name: "id",
					re:   regexp.MustCompile(`\d{3}`),
				}},
			}},
			map[string]string{"id": "123"},
			false,
			"123",
			false,
		},
		{
			"pattern pattern",
			&Template{slices: []_TemplateSlice{
				{valuePattern: &_ValuePattern{
					name: "name",
					re:   regexp.MustCompile(`[A-Za-z]{3}`),
				}},
				{valuePattern: &_ValuePattern{
					name: "id",
					re:   regexp.MustCompile(`\d{3}`),
				}},
			}},
			map[string]string{"name": "abc", "id": "123"},
			false,
			"abc123",
			false,
		},
		{
			"static pattern static pattern",
			&Template{slices: []_TemplateSlice{
				{staticStr: "name: "},
				{valuePattern: &_ValuePattern{
					name: "name",
					re:   regexp.MustCompile(`[A-Za-z]{3}`),
				}},
				{staticStr: ", id: "},
				{valuePattern: &_ValuePattern{
					name: "id",
					re:   regexp.MustCompile(`\d{3}`),
				}},
			}},
			map[string]string{"name": "abc", "id": "123"},
			false,
			"name: abc, id: 123",
			false,
		},
		{
			"static pattern static wildcard",
			&Template{slices: []_TemplateSlice{
				{staticStr: "name: "},
				{valuePattern: &_ValuePattern{
					name: "name",
					re:   regexp.MustCompile(`[A-Za-z]{3}`),
				}},
				{staticStr: ", address: "},
				{valuePattern: &_ValuePattern{
					name: "address",
				}},
			}},
			map[string]string{"name": "abc", "address": "Kepler-62e"},
			false,
			"name: abc, address: Kepler-62e",
			false,
		},
		{
			"static pattern static wildcard static pattern",
			&Template{slices: []_TemplateSlice{
				{staticStr: "id: "},
				{valuePattern: &_ValuePattern{
					name: "id",
					re:   regexp.MustCompile(`\d{3}`),
				}},
				{staticStr: ", address: "},
				{valuePattern: &_ValuePattern{
					name: "address",
				}},
				{staticStr: ", state: "},
				{valuePattern: &_ValuePattern{
					name: "state",
					re:   regexp.MustCompile(`(unknown|active|dormant)`),
				}},
			}},
			map[string]string{
				"id":      "123",
				"address": "unknown",
				"state":   "unknown",
			},
			false,
			"id: 123, address: unknown, state: unknown",
			false,
		},
		{
			"wildcard static pattern",
			&Template{slices: []_TemplateSlice{
				{valuePattern: &_ValuePattern{
					name: "galaxy",
				}},
				{staticStr: ", "},
				{valuePattern: &_ValuePattern{
					name: "color",
					re:   regexp.MustCompile(`(red|blue|white)`),
				}},
			}},
			map[string]string{"galaxy": "Eye of Sauron", "color": "red"},
			false,
			"Eye of Sauron, red",
			false,
		},
		{
			"static pattern static wildcard static pattern",
			&Template{slices: []_TemplateSlice{
				{staticStr: "id: "},
				{valuePattern: &_ValuePattern{
					name: "id",
					re:   regexp.MustCompile(`\d{3}`),
				}},
				{staticStr: ", address: "},
				{valuePattern: &_ValuePattern{
					name: "address",
				}},
				{staticStr: ", state: "},
				{valuePattern: &_ValuePattern{
					name: "state",
					re:   regexp.MustCompile(`(unknown|active|dormant)`),
				}},
			}},
			map[string]string{
				"id": "123",
			},
			true,
			"id: 123, address: , state: ",
			false,
		},
		{
			"static pattern static wildcard static pattern",
			&Template{slices: []_TemplateSlice{
				{staticStr: "id: "},
				{valuePattern: &_ValuePattern{
					name: "id",
					re:   regexp.MustCompile(`\d{3}`),
				}},
				{staticStr: ", address: "},
				{valuePattern: &_ValuePattern{
					name: "address",
				}},
				{staticStr: ", state: "},
				{valuePattern: &_ValuePattern{
					name: "state",
					re:   regexp.MustCompile(`(unknown|active|dormant)`),
				}},
			}},
			map[string]string{
				"id": "123",
			},
			false,
			"",
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.tmpl.Apply(c.values, c.ignoreMissing)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"Template.Apply() error = %v, wantErr %v",
					err,
					c.wantErr,
				)
			}

			if got != c.resultStr {
				t.Fatalf("Template.Apply() = %v, want %v", got, c.resultStr)
			}
		})
	}
}

func TestTemplate_String(t *testing.T) {
	var cases = []struct {
		name string
		tmpl *Template
		want string
	}{
		{
			"static",
			&Template{
				name:   "static",
				slices: []_TemplateSlice{{staticStr: "static"}},
			},
			"$static:static",
		},
		{
			"wildcard",
			&Template{
				name: "wildcard",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{name: "galaxy"}},
				},
			},
			"$wildcard:{galaxy}",
		},
		{
			"pattern",
			&Template{
				name: "pattern",
				slices: []_TemplateSlice{
					{
						valuePattern: &_ValuePattern{
							"id",
							regexp.MustCompile(`\d+`),
						},
					},
				},
			},
			`$pattern:{id:\d+}`,
		},
		{
			"pattern pattern",
			&Template{
				name: "pattern(2x)",
				slices: []_TemplateSlice{
					{
						valuePattern: &_ValuePattern{
							"name",
							regexp.MustCompile(`[A-Za-z]+`),
						},
					},
					{
						valuePattern: &_ValuePattern{
							"id",
							regexp.MustCompile(`\d+`),
						},
					},
				},
			},
			`$pattern(2x):{name:[A-Za-z]+}{id:\d+}`,
		},
		{
			"static pattern static pattern",
			&Template{
				name: "static: pattern (2x)",
				slices: []_TemplateSlice{
					{staticStr: "name: "},
					{
						valuePattern: &_ValuePattern{
							"name",
							regexp.MustCompile(`[A-Za-z]+`),
						},
					},
					{staticStr: ", id: "},
					{
						valuePattern: &_ValuePattern{
							"id",
							regexp.MustCompile(`\d+`),
						},
					},
					{staticStr: "{*}"},
				},
			},
			`$static\: pattern (2x):name: {name:[A-Za-z]+}, id: {id:\d+}\{*\}`,
		},
		{
			"static wildcard static pattern static",
			&Template{
				name: "$template",
				slices: []_TemplateSlice{
					{staticStr: `name{first}: `},
					{
						valuePattern: &_ValuePattern{name: "name::first"},
					},
					{staticStr: `,{id:`},
					{
						valuePattern: &_ValuePattern{
							"id::digit",
							regexp.MustCompile(`\d+`),
						},
					},
					{staticStr: "}"},
				},
			},
			`$$template:name\{first\}: {name\:\:first},\{id:{id\:\:digit:\d+}\}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tmpl.String(); got != c.want {
				t.Fatalf("Template.String() = %v, want %v", got, c.want)
			}
		})
	}
}

func Test_templateNameAndContent(t *testing.T) {
	var cases = []struct {
		name            string
		tmplStr         string
		wantName        string
		wantTmplContent string
		wantErr         bool
	}{
		{
			`$tmpl\:1`,
			`$tmpl\:1:static{valueName:pattern}`,
			`tmpl:1`,
			"static{valueName:pattern}",
			false,
		},
		{
			`empty #1 - \$tmpl\:`,
			`\$tmpl\:1 static{valueName:pattern}`,
			"",
			`$tmpl\:1 static{valueName:pattern}`,
			false,
		},
		{
			"empty #2 - tmpl",
			"tmpl:2 static{valueName:pattern}",
			"",
			"tmpl:2 static{valueName:pattern}",
			false,
		},
		{
			"empty #3 - $:",
			"$:static{valueName:pattern}",
			"",
			"static{valueName:pattern}",
			false,
		},
		{
			"empty #4 - $:",
			"$:",
			"",
			"",
			false,
		},
		{
			"error #2 - $",
			"$",
			"",
			"",
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gotName, gotTmplContent, err = templateNameAndContent(
				c.tmplStr,
			)

			if (err != nil) != c.wantErr {
				t.Fatalf(
					"templateNameAndContent() error = %v, wantErr %v",
					err,
					c.wantErr,
				)

				return
			}

			if gotName != c.wantName {
				t.Fatalf(
					"templateNameAndContent() gotName = %v, want %v",
					gotName,
					c.wantName,
				)
			}

			if gotTmplContent != c.wantTmplContent {
				t.Fatalf(
					"templateNameAndContent() gotTmplContent = %v, want %v", gotTmplContent,
					c.wantTmplContent,
				)
			}
		})
	}
}

func Test_staticSlice(t *testing.T) {
	var cases = []struct {
		tmplStr         string
		wantStaticStr   string
		wantLeftTmplStr string
		wantErr         bool
	}{
		{"static", "static", "", false},
		{"static{valueName:pattern}", "static", "{valueName:pattern}", false},
		{`static\{valueName:pattern\}`, "static{valueName:pattern}", "", false},
		{"{valueName:pattern}", "", "{valueName:pattern}", false},
		{`static\{}}{valueName:pattern}`, "", "", true},
	}

	for _, c := range cases {
		t.Run(c.tmplStr, func(t *testing.T) {
			var staticStr, leftTmplStr, err = staticSlice(c.tmplStr)

			if (err != nil) != c.wantErr {
				t.Fatalf(
					"staticSlice() error = %v, wantErr %v",
					err,
					c.wantErr,
				)

				return
			}

			if staticStr != c.wantStaticStr {
				t.Fatalf(
					"staticSlice() gotStr = %v, want %v",
					staticStr,
					c.wantStaticStr,
				)
			}

			if leftTmplStr != c.wantLeftTmplStr {
				t.Fatalf(
					"staticSlice() leftTmplStr = %v, want %v",
					leftTmplStr,
					c.wantLeftTmplStr,
				)
			}
		})
	}
}

func Test_dynamicSlice(t *testing.T) {
	var cases = []struct {
		tmplStr         string
		wantValueName   string
		wantPattern     string
		wantLeftTmplStr string
		wantErr         bool
	}{
		{
			"{valueName:pattern}",
			"valueName", "pattern", "",
			false,
		},
		{
			`{valueName\:1:pattern}`,
			`valueName:1`, "pattern", "",
			false,
		},
		{
			`{\:valueName\::pattern}{valueName:pattern}`,
			":valueName:", "pattern", "{valueName:pattern}",
			false,
		},
		{
			`{value{Name}:pattern}{valueName:pattern}`,
			"value{Name}", "pattern", "{valueName:pattern}",
			false,
		},
		{
			`{valueName:pattern} static`,
			"valueName", "pattern", " static",
			false,
		},
		{
			`{valueName}:pattern\{Name\}:pattern{valueName:pattern}`,
			"valueName", "", `:pattern\{Name\}:pattern{valueName:pattern}`,
			false,
		},
		{
			`(valueName:} static`,
			"", "", "",
			true,
		},
		{
			`{:pattern} static`,
			"", "", "",
			true,
		},
		{
			`{value{Name:patternName}:{pattern}`,
			"", "", "",
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.tmplStr, func(t *testing.T) {
			var valueName, pattern, leftTmplStr, err = dynamicSlice(
				c.tmplStr,
			)

			if valueName != c.wantValueName {
				t.Fatalf(
					"dynamicSlice() valueName = %v, want %v",
					valueName,
					c.wantValueName,
				)
			}

			if pattern != c.wantPattern {
				t.Fatalf(
					"dynamicSlice() pattern = %v, want %v",
					pattern,
					c.wantPattern,
				)
			}

			if leftTmplStr != c.wantLeftTmplStr {
				t.Fatalf(
					"dynamicSlice() leftTmplStr = %v, want %v",
					leftTmplStr,
					c.wantLeftTmplStr,
				)
			}

			if (err != nil) != c.wantErr {
				t.Fatalf("dynamicSlice() err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

func Test_appendDynamicSliceTo(t *testing.T) {
	var (
		tss           []_TemplateSlice
		wildCardIdx   = -1
		valuePatterns = make(map[string]*_ValuePattern)
	)

	var cases = []struct {
		name            string
		vName           string
		pattern         string
		wantTss         []_TemplateSlice
		wantWildCardIdx int
		wantErr         bool
	}{
		{
			"name #1", "name", "[A-Za-z]{2,8}",
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
			},
			-1, false,
		},
		{
			"id #1", "id", `\d{3}`,
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
			},
			-1, false,
		},
		{
			"name #2", "name", "",
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
			},
			-1, false,
		},
		{
			"id #2", "id", "",
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
			},
			-1, false,
		},
		{
			"address #1", "address", "",
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{valuePattern: &_ValuePattern{name: "address"}},
			},
			4, false,
		},
		{
			"color #1", "color", "(red|green|blue)",
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{valuePattern: &_ValuePattern{name: "address"}},
				{
					valuePattern: &_ValuePattern{
						"color",
						regexp.MustCompile(`(red|green|blue)$`),
					},
				},
			},
			4, false,
		},
		{
			"name #3", "name", "",
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{valuePattern: &_ValuePattern{name: "address"}},
				{
					valuePattern: &_ValuePattern{
						"color",
						regexp.MustCompile(`(red|green|blue)$`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile(`[A-Za-z]{2,8}$`),
					},
				},
			},
			4, false,
		},
		{
			"id #3", "id", "",
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile("^[A-Za-z]{2,8}"),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`^\d{3}`),
					},
				},
				{valuePattern: &_ValuePattern{name: "address"}},
				{
					valuePattern: &_ValuePattern{
						"color",
						regexp.MustCompile(`(red|green|blue)$`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"name",
						regexp.MustCompile(`[A-Za-z]{2,8}$`),
					},
				},
				{
					valuePattern: &_ValuePattern{
						"id",
						regexp.MustCompile(`\d{3}$`),
					},
				},
			},
			4, false,
		},
		{"error #1", "city", "", nil, -1, true},
		{"error #2", "", "", nil, -1, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err error
			tss, wildCardIdx, err = appendDynamicSliceTo(
				tss,
				c.vName, c.pattern,
				valuePatterns, wildCardIdx,
			)

			if (err != nil) != c.wantErr {
				t.Fatalf(
					"appendDynamicSliceTo() error = %v, wantErr %v",
					err,
					c.wantErr,
				)

				return
			}

			var tmpl = Template{slices: tss, wildCardIdx: wildCardIdx}
			var wantTmpl = Template{
				slices:      c.wantTss,
				wildCardIdx: c.wantWildCardIdx,
			}

			if !reflect.DeepEqual(tss, c.wantTss) {
				t.Fatalf(
					"appendDynamicSliceTo() tss = %s, want %s",
					tmpl.String(),
					wantTmpl.String(),
				)
			}

			if wildCardIdx != c.wantWildCardIdx {
				t.Fatalf(
					"appendDynamicSliceTo() wildCardIdx = %v, want %v",
					wildCardIdx,
					c.wantWildCardIdx,
				)
			}
		})
	}
}

func Test_parse(t *testing.T) {
	var cases = []struct {
		name            string
		tmplStr         string
		wantTmplSlcs    []_TemplateSlice
		wantWildCardIdx int
		wantErr         bool
	}{
		{
			"static",
			`static\{slice\}`,
			[]_TemplateSlice{{staticStr: `static{slice}`}},
			-1, false,
		},
		{
			"pattern",
			`{valueName:pattern}`,
			[]_TemplateSlice{
				{
					valuePattern: &_ValuePattern{
						name: "valueName",
						re:   regexp.MustCompile("^pattern$"),
					},
				},
			},
			-1, false,
		},
		{
			"wildcard",
			`{wildcard\:slice}`,
			[]_TemplateSlice{
				{valuePattern: &_ValuePattern{name: "wildcard:slice"}},
			},
			0, false,
		},
		{
			"static pattern",
			`static\{slice\} {\:valueName\::pattern}`,
			[]_TemplateSlice{
				{staticStr: `static{slice} `},
				{valuePattern: &_ValuePattern{
					name: `:valueName:`,
					re:   regexp.MustCompile("^pattern"), // ^pattern$   ???
				}},
			},
			-1, false,
		},
		{
			"static wildcard pattern",
			`static{\:wildcard\:}{valueName:pattern}`,
			[]_TemplateSlice{
				{staticStr: "static"},
				{valuePattern: &_ValuePattern{name: ":wildcard:"}},
				{valuePattern: &_ValuePattern{
					name: "valueName",
					re:   regexp.MustCompile("pattern$"),
				}},
			},
			1, false,
		},
		{
			"wildcard static pattern",
			`{wildcard}static{valueName:pattern}`,
			[]_TemplateSlice{
				{valuePattern: &_ValuePattern{name: "wildcard"}},
				{staticStr: "static"},
				{valuePattern: &_ValuePattern{
					name: "valueName",
					re:   regexp.MustCompile("pattern$"),
				}},
			},
			0, false,
		},
		{
			"static pattern wildcard",
			`static{value {name}:pattern}{wildcard}`,
			[]_TemplateSlice{
				{staticStr: "static"},
				{valuePattern: &_ValuePattern{
					name: "value {name}",
					re:   regexp.MustCompile("^pattern"),
				}},
				{valuePattern: &_ValuePattern{name: "wildcard"}},
			},
			2, false,
		},
		{
			"pattern pattern static",
			`{valueName1:pattern1}{valueName2:pattern2} static`,
			[]_TemplateSlice{
				{valuePattern: &_ValuePattern{
					name: "valueName1",
					re:   regexp.MustCompile("^pattern1"),
				}},
				{valuePattern: &_ValuePattern{
					name: "valueName2",
					re:   regexp.MustCompile("^pattern2"),
				}},
				{staticStr: " static"},
			},
			-1, false,
		},
		{
			"pattern pattern wildcard static pattern pattern static pattern",
			`{valueName1:pattern1}{valueName2:pattern2}{wildcard} static1 {valueName1}{valueName3:pattern3} static2:{valueName2}`,
			[]_TemplateSlice{
				{valuePattern: &_ValuePattern{
					name: "valueName1",
					re:   regexp.MustCompile("^pattern1"),
				}},
				{valuePattern: &_ValuePattern{
					name: "valueName2",
					re:   regexp.MustCompile("^pattern2"),
				}},
				{valuePattern: &_ValuePattern{name: "wildcard"}},
				{staticStr: " static1 "},
				{valuePattern: &_ValuePattern{
					name: "valueName1",
					re:   regexp.MustCompile("pattern1$"),
				}},
				{valuePattern: &_ValuePattern{
					name: "valueName3",
					re:   regexp.MustCompile("pattern3$"),
				}},
				{staticStr: " static2:"},
				{valuePattern: &_ValuePattern{
					name: "valueName2",
					re:   regexp.MustCompile("pattern2$"),
				}},
			},
			2, false,
		},
		{
			"wildcard pattern wildcard-1",
			`{wildcard}{valueName:pattern}{wildcard}`,
			nil, -1, true,
		},
		{
			"wildcard pattern wildcard-2",
			`{wildcard}{valueName:pattern}{city}`,
			nil, -1, true,
		},
		{
			"wildcard pattern pattern(error)-1",
			`{wildcard}{valueName1:pattern}{valueName2:}`,
			nil, -1, true,
		},
		{
			"wildcard pattern pattern(error)-2",
			`{wildcard}{valueName:pattern}{:city}`,
			nil, -1, true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gotTmplSlcs, gotWildCardIdx, err = parse(c.tmplStr)
			if (err != nil) != c.wantErr {
				t.Fatalf("parse() error = %v, wantErr %v", err, c.wantErr)
				return
			}

			var gotTmpl = Template{
				slices:      gotTmplSlcs,
				wildCardIdx: gotWildCardIdx,
			}

			var wantTmpl = Template{
				slices:      c.wantTmplSlcs,
				wildCardIdx: c.wantWildCardIdx,
			}

			if !reflect.DeepEqual(gotTmplSlcs, c.wantTmplSlcs) {
				t.Fatalf(
					"parse() gotTmplSlcs = %s, want %s",
					gotTmpl.String(),
					wantTmpl.String(),
				)
			}

			if gotWildCardIdx != c.wantWildCardIdx {
				t.Fatalf(
					"parse() gotWildCardIdx = %v, want %v",
					gotWildCardIdx,
					c.wantWildCardIdx,
				)
			}
		})
	}
}

func TestTryToParse(t *testing.T) {
	var cases = []struct {
		name    string
		tmplStr string
		want    *Template
		wantErr bool
	}{

		{
			"static",
			"$name:static",
			&Template{
				name:        "name",
				slices:      []_TemplateSlice{{staticStr: "static"}},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"wildcard",
			"{wildcard}",
			&Template{
				name: "wildcard",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{name: "wildcard"}},
				},
				wildCardIdx: 0,
			},
			false,
		},
		{
			"static wildcard static",
			"static1 {wildcard} static2",
			&Template{
				name: "wildcard",
				slices: []_TemplateSlice{
					{staticStr: "static1 "},
					{valuePattern: &_ValuePattern{name: "wildcard"}},
					{staticStr: " static2"},
				},
				wildCardIdx: 1,
			},
			false,
		},
		{
			"static pattern static",
			"static1 {name:pattern} static2",
			&Template{
				name: "name",
				slices: []_TemplateSlice{
					{staticStr: "static1 "},
					{valuePattern: &_ValuePattern{
						name: "name",
						re:   regexp.MustCompile("^pattern"),
					}},
					{staticStr: " static2"},
				},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"wildcard",
			"$name:{wildcard}",
			&Template{
				name: "name",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{name: "wildcard"}},
				},
				wildCardIdx: 0,
			},
			false,
		},
		{
			"static pattern",
			`static\{slice\} {\:valueName\::pattern}`,
			&Template{
				name: ":valueName:",
				slices: []_TemplateSlice{
					{staticStr: "static{slice} "},
					{valuePattern: &_ValuePattern{
						name: `:valueName:`,
						re:   regexp.MustCompile("^pattern"),
					}},
				},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"static wildcard pattern",
			`$name:static{\:wildcard\:}{valueName:pattern}`,
			&Template{
				name: "name",
				slices: []_TemplateSlice{
					{staticStr: "static"},
					{valuePattern: &_ValuePattern{name: ":wildcard:"}},
					{valuePattern: &_ValuePattern{
						name: "valueName",
						re:   regexp.MustCompile("pattern$"),
					}},
				},
				wildCardIdx: 1,
			},
			false,
		},
		{
			"wildcard static pattern",
			`$$\:name:{wildcard}static{valueName:pattern}`,
			&Template{
				name: "$:name",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{name: "wildcard"}},
					{staticStr: "static"},
					{valuePattern: &_ValuePattern{
						name: "valueName",
						re:   regexp.MustCompile("pattern$"),
					}},
				},
				wildCardIdx: 0,
			},
			false,
		},
		{
			"static pattern wildcard",
			`static{value {name}:pattern}{wildcard}`,
			&Template{
				slices: []_TemplateSlice{
					{staticStr: "static"},
					{valuePattern: &_ValuePattern{
						name: "value {name}",
						re:   regexp.MustCompile("^pattern"),
					}},
					{valuePattern: &_ValuePattern{name: "wildcard"}},
				},
				wildCardIdx: 2,
			},
			false,
		},
		{
			"pattern pattern static",
			`{valueName1:pattern1}{valueName1} static`,
			&Template{
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{
						name: "valueName1",
						re:   regexp.MustCompile("^pattern1"),
					}},
					{valuePattern: &_ValuePattern{
						name: "valueName1",
						re:   regexp.MustCompile("^pattern1"),
					}},
					{staticStr: " static"},
				},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"wildcard pattern wildcard-1",
			`{wildcard}{valueName:pattern}{wildcard}`,
			nil, true,
		},
		{
			"wildcard pattern wildcard-2",
			`{wildcard}{valueName:pattern}{city}`,
			nil, true,
		},
		{
			"wildcard pattern pattern(error)-1",
			`{wildcard}{valueName:pattern}{address:}`,
			nil, true,
		},
		{
			"wildcard pattern pattern(error)-2",
			`{wildcard}{valueName:pattern}{:city}`,
			nil, true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got, err = TryToParse(c.tmplStr)
			if (err != nil) != c.wantErr {
				t.Fatalf("Parse() error = %v, wantErr %v", err, c.wantErr)
				return
			}

			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Parse() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	var cases = []struct {
		name    string
		tmplStr string
		want    *Template
		wantErr bool
	}{

		{
			"static",
			"$name:static",
			&Template{
				name:        "name",
				slices:      []_TemplateSlice{{staticStr: "static"}},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"wildcard",
			"{wildcard}",
			&Template{
				name: "wildcard",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{name: "wildcard"}},
				},
				wildCardIdx: 0,
			},
			false,
		},
		{
			"static wildcard static",
			"static1 {wildcard} static2",
			&Template{
				name: "wildcard",
				slices: []_TemplateSlice{
					{staticStr: "static1 "},
					{valuePattern: &_ValuePattern{name: "wildcard"}},
					{staticStr: " static2"},
				},
				wildCardIdx: 1,
			},
			false,
		},
		{
			"static pattern static",
			"static1 {name:pattern} static2",
			&Template{
				name: "name",
				slices: []_TemplateSlice{
					{staticStr: "static1 "},
					{valuePattern: &_ValuePattern{
						name: "name",
						re:   regexp.MustCompile("^pattern"),
					}},
					{staticStr: " static2"},
				},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"wildcard",
			"$name:{wildcard}",
			&Template{
				name: "name",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{name: "wildcard"}},
				},
				wildCardIdx: 0,
			},
			false,
		},
		{
			"static pattern",
			`static\{slice\} {\:valueName\::pattern}`,
			&Template{
				name: ":valueName:",
				slices: []_TemplateSlice{
					{staticStr: "static{slice} "},
					{valuePattern: &_ValuePattern{
						name: `:valueName:`,
						re:   regexp.MustCompile("^pattern"),
					}},
				},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"static wildcard pattern",
			`$name:static{\:wildcard\:}{valueName:pattern}`,
			&Template{
				name: "name",
				slices: []_TemplateSlice{
					{staticStr: "static"},
					{valuePattern: &_ValuePattern{name: ":wildcard:"}},
					{valuePattern: &_ValuePattern{
						name: "valueName",
						re:   regexp.MustCompile("pattern$"),
					}},
				},
				wildCardIdx: 1,
			},
			false,
		},
		{
			"wildcard static pattern",
			`$$\:name:{wildcard}static{valueName:pattern}`,
			&Template{
				name: "$:name",
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{name: "wildcard"}},
					{staticStr: "static"},
					{valuePattern: &_ValuePattern{
						name: "valueName",
						re:   regexp.MustCompile("pattern$"),
					}},
				},
				wildCardIdx: 0,
			},
			false,
		},
		{
			"static pattern wildcard",
			`static{value {name}:pattern}{wildcard}`,
			&Template{
				slices: []_TemplateSlice{
					{staticStr: "static"},
					{valuePattern: &_ValuePattern{
						name: "value {name}",
						re:   regexp.MustCompile("^pattern"),
					}},
					{valuePattern: &_ValuePattern{name: "wildcard"}},
				},
				wildCardIdx: 2,
			},
			false,
		},
		{
			"pattern pattern static",
			`{valueName1:pattern1}{valueName1} static`,
			&Template{
				slices: []_TemplateSlice{
					{valuePattern: &_ValuePattern{
						name: "valueName1",
						re:   regexp.MustCompile("^pattern1"),
					}},
					{valuePattern: &_ValuePattern{
						name: "valueName1",
						re:   regexp.MustCompile("^pattern1"),
					}},
					{staticStr: " static"},
				},
				wildCardIdx: -1,
			},
			false,
		},
		{
			"wildcard pattern wildcard-1",
			`{wildcard}{valueName:pattern}{wildcard}`,
			nil, true,
		},
		{
			"wildcard pattern wildcard-2",
			`{wildcard}{valueName:pattern}{city}`,
			nil, true,
		},
		{
			"wildcard pattern pattern(error)-1",
			`{wildcard}{valueName:pattern}{address:}`,
			nil, true,
		},
		{
			"wildcard pattern pattern(error)-2",
			`{wildcard}{valueName:pattern}(:city}`,
			nil, true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got *Template
			defer func() {
				// t.Helper()
				var err = recover()
				if (err != nil) != c.wantErr {
					t.Fatalf("Parse() error = %v, wantErr %v", err, c.wantErr)
					return
				}

				if !reflect.DeepEqual(got, c.want) {
					t.Fatalf("Parse() = %v, want %v", got, c.want)
				}
			}()

			got = Parse(c.tmplStr)
		})
	}
}
