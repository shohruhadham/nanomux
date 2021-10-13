// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"regexp"
	"strings"
)

// --------------------------------------------------

// Similarity is a degree of difference between templates :)
type Similarity uint8

const (
	// Different templates have different static and pattern parts.
	Different Similarity = iota

	// DifferentValueNames means templates have the same static and/or pattern
	// parts, but have different value names for their patterns.
	DifferentValueNames

	// DifferentNames means templates are the same except their names.
	DifferentNames

	// TheSame templates have no differences.
	TheSame
)

var ErrDifferentTemplates = fmt.Errorf("different templates")
var ErrDifferentValueNames = fmt.Errorf("different value names")
var ErrDifferentNames = fmt.Errorf("different names")

// Err returns differences as errors.
func (s Similarity) Err() error {
	switch s {
	case Different:
		return ErrDifferentTemplates
	case DifferentValueNames:
		return ErrDifferentValueNames
	case DifferentNames:
		return ErrDifferentNames
	case TheSame:
		return nil
	default:
		panic(fmt.Errorf("undefined similarity"))
	}
}

// --------------------------------------------------

// ErrInvalidTemplate is returned when a template is empty or not complete.
var ErrInvalidTemplate = fmt.Errorf("invalid template")

// ErrInvalidValue is returned from the Template's Apply method when one of the
// values didn't match the pattern.
var ErrInvalidValue = fmt.Errorf("invalid value")

// ErrMissingValue is returned from the Template's Apply method when one of the
// values is missing.
var ErrMissingValue = fmt.Errorf("missing value")

// ErrDifferentPattern is returned when a different pattern is provided for the
// repeated value name.
var ErrDifferentPattern = fmt.Errorf("different pattern")

// ErrRepeatedWildCardName is returned when the wild card name comes again in
// the template.
var ErrRepeatedWildCardName = fmt.Errorf("repeated wild card name")

// ErrAnotherWildCardName is returned when there is more than one wild card name
// in the template.
var ErrAnotherWildCardName = fmt.Errorf("another wild card name")

type _ValuePattern struct {
	name string
	re   *regexp.Regexp
}

type _TemplateSlice struct {
	staticStr    string         // Static slice of the template.
	valuePattern *_ValuePattern // Name-pattern slice of the template.
}

// --------------------------------------------------

// Template represents the parsed template of the hosts and resources.
//
// Value-pattern and wild card parts are the dynamic slices of the template.
// If there is a single dynamic slice in the template and the template doesn't
// have a name, dynamic slice's name is used as the name of the template.
//
// There can be only one wild card dynamic slice in the template.
//
// If the value-pattern part is repeated in the template its pattern may be
// omitted. When the template is matching a string its repeated value-pattern
// must get the same value, otherwise matching fails.
//
// Colon ':' in the template name and the value name, as well as the curly
// braces '{', '}' in the static part can be escaped with the backslash '\'.
// Escaped colon ':' is included in the name and the escaped curly braces '{',
// '}' are included in the static part. If the static part at the beginning of
// the template starts with the '$' sign, it must be escaped too.
//
// Some examples of the template forms:
//
// 	$templateName:staticPart{valueName:pattern},
// 	$templateName:{valueName:pattern}staticpart,
// 	$templateName:{wildCardName}{valueName1:pattern1}{valueName2:pattern2},
// 	staticTemplate,
// 	{valueName:pattern},
// 	{wildCardName},
// 	{valueName:pattern}staticPart{wildCardName}{valueName},
// 	$templateName:staticPart1{wildCardName}staticPart2{valueName:pattern}
// 	$templateName:staticPart,
// 	$templateName:{valueName:pattern},
// 	$templateName\:1:{wildCard{Name}}staticPart{value{Name}:pattern},
// 	{valueName\:1:pattern}static\{Part\},
// 	\$staticPart:1{wildCardName}staticPart:2
type Template struct {
	name        string
	slices      []_TemplateSlice
	wildCardIdx int
}

// -----

// SetName sets the name of the template. Name becomes the name of the host
// or resource.
func (t *Template) SetName(name string) {
	t.name = name
}

// Name returns the name of the template.
func (t *Template) Name() string {
	return t.name
}

// Content returns the content of the template without a name.
// Pattern is omitted from a repeated value-pattern starting from the second
// repitition.
func (t *Template) Content() string {
	var (
		strb    = strings.Builder{}
		vns     = make(map[string]bool)
		tslices = t.slices
	)

	if t.name == "" && len(tslices) != 0 {
		if tslices[0].staticStr != "" && tslices[0].staticStr[0] == '$' {
			strb.WriteByte('\\')
		}
	}

	for _, slice := range t.slices {
		if slice.staticStr != "" {
			var idx = 0
			for i, ch := range slice.staticStr {
				if ch == '{' || ch == '}' {
					strb.WriteString(slice.staticStr[idx:i])
					strb.WriteByte('\\')
					strb.WriteRune(ch)
					idx = i + 1
				}
			}

			strb.WriteString(slice.staticStr[idx:len(slice.staticStr)])
		} else {
			strb.WriteByte('{')
			for str := slice.valuePattern.name; len(str) > 0; {
				var idx = strings.Index(str, ":")
				if idx < 0 {
					strb.WriteString(str)
					break
				}

				strb.WriteString(str[:idx])
				strb.WriteString(`\:`)
				str = str[idx+1:]
			}

			if slice.valuePattern.re != nil && !vns[slice.valuePattern.name] {
				strb.WriteByte(':')
				strb.WriteString(slice.valuePattern.re.String())
				vns[slice.valuePattern.name] = true
			}

			strb.WriteByte('}')
		}
	}

	return strb.String()
}

// IsStatic returns true if the template doesn't have any pattern or a wild card
// part.
func (t *Template) IsStatic() bool {
	return len(t.slices) == 1 && t.slices[0].staticStr != ""
}

// IsWildCard returns true if the template doesn't have any static or pattern
// parts.
func (t *Template) IsWildCard() bool {
	if len(t.slices) == 1 {
		var vp = t.slices[0].valuePattern
		if vp != nil && vp.re == nil {
			return true
		}
	}

	return false
}

// HasPattern returns true if the template has any value-pattern part.
func (t *Template) HasPattern() bool {
	var lslices = len(t.slices)
	for i := 0; i < lslices; i++ {
		if t.slices[i].valuePattern.re != nil {
			return true
		}
	}

	return false
}

// SimilarityWith returns the similarity between the receiver and argument
// templates.
func (t *Template) SimilarityWith(anotherT *Template) Similarity {
	if anotherT == nil {
		panic(ErrNilArgument)
	}

	if t.IsStatic() {
		if anotherT.IsStatic() {
			if t.slices[0].staticStr == anotherT.slices[0].staticStr {
				if t.name != anotherT.name {
					return DifferentNames
				}

				return TheSame
			}
		}

		return Different
	}

	if t.IsWildCard() {
		if anotherT.IsWildCard() {
			if t.slices[0].valuePattern.name ==
				anotherT.slices[0].valuePattern.name {
				if t.name != anotherT.name {
					return DifferentNames
				}

				return TheSame
			}

			return DifferentValueNames
		}

		return Different
	}

	if anotherT.IsStatic() || anotherT.IsWildCard() {
		return Different
	}

	if t.wildCardIdx != anotherT.wildCardIdx {
		return Different
	}

	var lts = len(t.slices)
	if lts != len(anotherT.slices) {
		return Different
	}

	var similarity = TheSame
	for i := 0; i < lts; i++ {
		var slc1, slc2 = t.slices[i], anotherT.slices[i]
		if slc1.staticStr != "" {
			if slc1.staticStr != slc2.staticStr {
				return Different
			}

			continue
		}

		if slc1.valuePattern.re != nil && slc2.valuePattern.re != nil {
			if slc1.valuePattern.re.String() != slc2.valuePattern.re.String() {
				return Different
			}

			if slc1.valuePattern.name != slc2.valuePattern.name {
				similarity = DifferentValueNames
			}
		} else if slc1.valuePattern.re == nil && slc2.valuePattern.re == nil {
			if slc1.valuePattern.name != slc2.valuePattern.name {
				similarity = DifferentValueNames
			}
		} else {
			return Different
		}
	}

	if similarity == TheSame && t.name != anotherT.name {
		similarity = DifferentNames
	}

	return similarity
}

// Match returns true if the string matches the template. If the template has
// value-pattern parts, Match also returns the values that matched patterns.
// Names of the patterns in the template are used as keys for the values.
func (t *Template) Match(str string) (matched bool, values map[string]string) {
	if t.IsStatic() {
		return t.slices[0].staticStr == str, nil
	}

	values = make(map[string]string)

	if t.IsWildCard() {
		values[t.slices[0].valuePattern.name] = str
		return true, values
	}

	var ltslices = len(t.slices)
	var k = ltslices
	if t.wildCardIdx >= 0 {
		k = t.wildCardIdx
	}

	for i := 0; i < k; i++ {
		var sstr = t.slices[i].staticStr
		if sstr != "" {
			if strings.HasPrefix(str, sstr) {
				str = str[len(sstr):]
			} else {
				return false, nil
			}
		} else {
			var vp = t.slices[i].valuePattern
			var idxs = vp.re.FindStringIndex(str)
			if idxs != nil {
				var v = str[:idxs[1]]
				if vf, found := values[vp.name]; found {
					if v != vf {
						return false, nil
					}
				} else {
					values[vp.name] = v
				}

				str = str[idxs[1]:]
			} else {
				return false, nil
			}
		}
	}

	for i := ltslices - 1; i > k; i-- {
		var sstr = t.slices[i].staticStr
		if sstr != "" {
			if strings.HasSuffix(str, sstr) {
				str = str[:len(str)-len(sstr)]
			} else {
				return false, nil
			}
		} else {
			var vp = t.slices[i].valuePattern
			var idxs = vp.re.FindAllStringIndex(str, -1)
			if idxs != nil {
				var lastIdxs = idxs[len(idxs)-1]
				var v = str[lastIdxs[0]:]
				if vf, found := values[vp.name]; found {
					if v != vf {
						return false, nil
					}
				} else {
					values[vp.name] = v
				}

				str = str[:lastIdxs[0]]
			} else {
				return false, nil
			}
		}
	}

	if t.wildCardIdx >= 0 && len(str) > 0 {
		var vpName = t.slices[t.wildCardIdx].valuePattern.name
		values[vpName] = str
	}

	return true, values
}

// Apply puts the values in the place of patterns if they match.
// When ignoreMissing is true, Apply ignores the missing values for the
// patterns instead of returning error.
func (t *Template) Apply(values map[string]string, ignoreMissing bool) (
	string,
	error,
) {
	var lslices = len(t.slices)
	var strb = strings.Builder{}

	for i := 0; i < lslices; i++ {
		var slc = t.slices[i]
		if slc.staticStr != "" {
			strb.WriteString(t.slices[i].staticStr)
			continue
		}

		if v, found := values[slc.valuePattern.name]; found {
			if slc.valuePattern.re != nil {
				var idxs = slc.valuePattern.re.FindStringIndex(v)
				if idxs == nil || (idxs[0] != 0 && idxs[1] != len(v)) {
					return "", newError(
						"%w value for %q",
						ErrInvalidValue,
						slc.valuePattern.name,
					)
				}
			}

			strb.WriteString(v)
		} else if ignoreMissing {
			continue
		} else {
			return "", newError(
				"%w for %q",
				ErrMissingValue,
				slc.valuePattern.name,
			)
		}
	}

	return strb.String(), nil
}

// String returns the template's string.
// Pattern is omitted from a repeated value-pattern starting from the second
// repitition.
func (t *Template) String() string {
	var strb = strings.Builder{}
	if t.name != "" {
		strb.WriteByte('$')
		var str = t.name
		for len(str) > 0 {
			var idx = strings.Index(str, ":")
			if idx < 0 {
				strb.WriteString(str)
				break
			}

			strb.WriteString(str[:idx])
			strb.WriteString(`\:`)
			str = str[idx+1:]
		}

		strb.WriteByte(':')
	}

	strb.WriteString(t.Content())
	return strb.String()
}

// --------------------------------------------------

// templateNameAndContent splits the template string into the name and content.
func templateNameAndContent(tmplStr string) (
	name string,
	content string,
	err error,
) {
	var ltmplStr = len(tmplStr)
	content = tmplStr

	if tmplStr[0] == '$' {
		if ltmplStr == 1 {
			return "", "", ErrInvalidTemplate
		}

		var strb = strings.Builder{}
		var idx = 1

		for i := 1; i < ltmplStr; i++ {
			idx = strings.IndexByte(tmplStr[i:], ':') + i
			if idx < i {
				strb.WriteString(tmplStr[i:])
				idx = ltmplStr - 1
			} else if idx > i {
				if tmplStr[idx-1] == '\\' {
					strb.WriteString(tmplStr[i : idx-1])
					strb.WriteByte(':')
					i = idx
					continue
				}

				strb.WriteString(tmplStr[i:idx])
			}

			break
		}

		name = strb.String()
		content = tmplStr[idx+1:]
		strb.Reset()
	} else if ltmplStr > 1 && tmplStr[0] == '\\' && tmplStr[1] == '$' {
		content = tmplStr[1:]
	}

	return
}

// staticSlice returns the static slice at the beginning of the template.
// If the template doesn't start with a static slice it's returned as is.
func staticSlice(tmplStrSlc string) (
	staticStr string,
	leftTmplStrSlc string,
	err error,
) {
	var (
		strb strings.Builder
		pch  rune = '0'
		idx       = 0
	)

	for i, ch := range tmplStrSlc {
		if ch == '{' {
			if pch != '\\' {
				strb.WriteString(tmplStrSlc[idx:i])
				staticStr = strb.String()
				leftTmplStrSlc = tmplStrSlc[i:]
				return
			}

			strb.WriteString(tmplStrSlc[idx : i-1])
			strb.WriteByte('{')
			idx = i + 1
		} else if ch == '}' {
			if pch != '\\' {
				err = newError(
					"%w - unescaped curly brace '}' at index %d",
					ErrInvalidTemplate,
					i,
				)

				return
			}

			strb.WriteString(tmplStrSlc[idx : i-1])
			strb.WriteByte('}')
			idx = i + 1
		}

		pch = ch
	}

	strb.WriteString(tmplStrSlc[idx:])
	staticStr = strb.String()
	return
}

// dynamicSlice returns the dynamic slice's value name and pattern at the
// beginning of the template. If the template doesn't start with a dynamic
// slice it's returned as is.
func dynamicSlice(tmplStrSlc string) (
	valueName, pattern, leftTmplStrSlc string,
	err error,
) {
	defer func() {
		if err != nil {
			valueName = ""
			pattern = ""
			leftTmplStrSlc = ""
		}
	}()

	var (
		sliceType   = 0 // 0-value name, 1-pattern
		depth       = 1
		ltmplStrSlc = len(tmplStrSlc)
		strb        = strings.Builder{}
	)

	for i, idx, chClsIdx := 1, 1, -1; i < ltmplStrSlc; i++ {
		var ch = tmplStrSlc[i]
		if ch == '{' {
			depth++
			continue
		}

		if sliceType == 0 {
			if ch == ':' {
				if i > 1 {
					// If the previous character is a backslash '\', current
					// character colon ':' is escaped. So, it's included in the
					// value name.
					if tmplStrSlc[i-1] == '\\' {
						strb.WriteString(tmplStrSlc[idx : i-1])
						strb.WriteByte(':')
						idx = i + 1
						continue
					}
				}

				if depth > 1 {
					err = newError("%w - open curly brace", ErrInvalidTemplate)
					return
				}

				strb.WriteString(tmplStrSlc[idx:i])
				if strb.Len() == 0 {
					err = newError("%w - empty value name", ErrInvalidTemplate)
					return
				}

				valueName = strb.String()
				strb.Reset()

				sliceType++
				idx = i + 1
				continue
			}

			if ch == '}' {
				depth--
				if depth > 0 {
					// Current curly brace '}' is not the end of the value name.
					continue
				}

				if strb.Len() > 0 {
					strb.WriteString(tmplStrSlc[idx:i])
					valueName = strb.String()
				} else {
					if i == idx {
						err = newError(
							"%w - empty dynamic slice",
							ErrInvalidTemplate,
						)

						return
					}

					valueName = tmplStrSlc[idx:i]
				}

				leftTmplStrSlc = tmplStrSlc[i+1:]

				// Here we have a value name without a pattern.
				return
			}
		}

		if sliceType == 1 {
			if ch == '\\' {
				i++
				// Backslack in a pattern escapes any character.
				continue
			}

			if chClsIdx >= 0 {
				if ch == ']' {
					var d = i - chClsIdx
					if d > 1 && !(d == 2 && tmplStrSlc[i-1] == '^') {
						// End of the character class.
						chClsIdx = -1
					}
				}

				continue
			}

			if ch == '[' {
				// Beginning of the character class.
				chClsIdx = i
				continue
			}

			if ch == '}' {
				depth--
				if depth > 0 {
					continue
				}

				if i == idx {
					err = newError("%w - empty pattern", ErrInvalidTemplate)
					return
				}

				pattern = tmplStrSlc[idx:i]
				leftTmplStrSlc = tmplStrSlc[i+1:]
				break
			}
		}
	}

	if depth > 0 {
		err = newError("%w - incomplete dynamic slice", ErrInvalidTemplate)
	}

	return
}

// appendDynamicSliceTo appends the value name and pattern to the list of
// dynamic slices. Map valuePatterns contains the previously created
// _ValuePattern instances with value names as a key. If the value name is
// repeated appendDynamicSliceTo reuses the _ValuePattern instance instead
// of creating a new one.
//
// If the passed argument wildCardIdx is the index of the previousy detected
// wild card, then it's returned as is. Otherwise, if the current dynamic slice
// is a wild card, its index in the list is returned.
func appendDynamicSliceTo(
	tss []_TemplateSlice,
	vName, pattern string,
	valuePatterns map[string]*_ValuePattern,
	wildCardIdx int,
) ([]_TemplateSlice, int, error) {
	if vp, exists := valuePatterns[vName]; exists {
		if pattern != "" {
			if wildCardIdx >= 0 {
				pattern += "$"
			} else {
				pattern = "^" + pattern
			}

			if pattern != vp.re.String() {
				return nil, -1, newError(
					"%w for a value %q",
					ErrDifferentPattern,
					vName,
				)
			}
		}

		// If value-pattern pair exists we don't have to create a new one.
		tss = append(tss, _TemplateSlice{valuePattern: vp})
		return tss, wildCardIdx, nil
	}

	if pattern == "" {
		if wildCardIdx >= 0 {
			var wc = tss[wildCardIdx]
			if vName == wc.valuePattern.name {
				return nil, -1, newError(
					"%w %q",
					ErrRepeatedWildCardName,
					vName,
				)
			}

			return nil, -1, newError("%w %q", ErrAnotherWildCardName, vName)
		}

		wildCardIdx = len(tss)
		tss = append(tss, _TemplateSlice{
			valuePattern: &_ValuePattern{name: vName},
		})

		// As the wild card slice has been appended, existing value-patterns
		// must be modified, so when they are reused template can match the
		// string from the end to the wild card slice.
		for _, vp := range valuePatterns {
			var p = vp.re.String()
			p = p[1:] + "$"

			var re, err = regexp.Compile(p)
			if err != nil {
				return nil, -1, err
			}

			valuePatterns[vp.name] = &_ValuePattern{vp.name, re}
		}

		return tss, wildCardIdx, nil
	}

	if wildCardIdx >= 0 {
		pattern += "$"
	} else {
		pattern = "^" + pattern
	}

	var re, err = regexp.Compile(pattern)
	if err != nil {
		return nil, -1, err
	}

	var vp = &_ValuePattern{name: vName, re: re}
	tss = append(tss, _TemplateSlice{valuePattern: vp})
	valuePatterns[vName] = vp

	return tss, wildCardIdx, nil
}

// $name:static{key1:pattern}static{key2:pattern}{key1}{key3}
// parse parses the template string and returns the template slices and the
// index of the wild card slice.
func parse(tmplStr string) (
	tmplSlcs []_TemplateSlice,
	wildCardIdx int,
	err error,
) {
	if tmplStr == "" {
		return nil, -1, newError("%w", ErrInvalidTemplate)
	}

	var (
		tmplStrSlc = tmplStr
		tss        = []_TemplateSlice{}

		valuePatterns = make(map[string]*_ValuePattern)
	)

	wildCardIdx = -1

	for len(tmplStrSlc) > 0 {
		var staticStr string
		staticStr, tmplStrSlc, err = staticSlice(tmplStrSlc)
		if err != nil {
			return nil, -1, err
		}

		if staticStr != "" {
			tss = append(tss, _TemplateSlice{staticStr: staticStr})
		}

		if tmplStrSlc == "" {
			break
		}

		var vName, pattern string
		vName, pattern, tmplStrSlc, err = dynamicSlice(tmplStrSlc)
		if err != nil {
			return nil, -1, err
		}

		tss, wildCardIdx, err = appendDynamicSliceTo(
			tss,
			vName, pattern,
			valuePatterns,
			wildCardIdx,
		)

		if err != nil {
			return nil, -1, err
		}
	}

	tmplSlcs = make([]_TemplateSlice, len(tss))
	copy(tmplSlcs, tss)

	if len(tmplSlcs) == 1 {
		if vp := tmplSlcs[0].valuePattern; vp != nil && vp.re != nil {
			// There is no other slices other than single value-pattern slice.
			// So, its pattern must be modified to match the whole string.
			var reStr = vp.re.String() + "$"
			vp.re, err = regexp.Compile(reStr)
			if err != nil {
				return nil, -1, err
			}
		}
	}

	return tmplSlcs, wildCardIdx, nil
}

// TryToParse tries to parse the passed template string, and if succeeds
// returns the Template instance.
func TryToParse(tmplStr string) (*Template, error) {
	if tmplStr == "" {
		return nil, newError(" %w - empty template", ErrInvalidTemplate)
	}

	var name string
	var err error
	name, tmplStr, err = templateNameAndContent(tmplStr)
	if err != nil {
		return nil, err
	}

	var tmpl = &Template{name: name}
	tmpl.slices, tmpl.wildCardIdx, err = parse(tmplStr)
	if err != nil {
		return nil, err
	}

	if !tmpl.IsStatic() && tmpl.name == "" {
		if tmpl.IsWildCard() {
			tmpl.name = tmpl.slices[0].valuePattern.name
		} else {
			var idx = -1
			for i, slc := range tmpl.slices {
				if slc.valuePattern != nil {
					if idx > -1 {
						// If there is a single dynamic slice in the template
						// and the template doesn't have a name, dynamic
						// slice's name is used as the name of the template.
						return tmpl, nil
					}

					idx = i
				}
			}

			if idx > -1 {
				tmpl.name = tmpl.slices[idx].valuePattern.name
			}
		}
	}

	return tmpl, nil
}

// Parse parses the template string and returns the Template instance if
// succeeds. Unlike TryToParse, Parse panics on error.
func Parse(tmplStr string) *Template {
	var tmpl, err = TryToParse(tmplStr)
	if err != nil {
		panic(err)
	}

	return tmpl
}
