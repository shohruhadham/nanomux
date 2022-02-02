// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"regexp"
	"strings"
)

// --------------------------------------------------

// Similarity is a degree of difference between templates. :)
type Similarity uint8

const (
	// Different templates have different segments.
	Different Similarity = iota

	// DifferentValueNames means templates have the same static and/or dynamic
	// segments but with different value names.
	DifferentValueNames

	// DifferentNames means that the templates are identical except for their
	// names.
	DifferentNames

	// TheSame templates have no differences.
	TheSame
)

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

type _ValuePattern struct {
	name string
	re   *regexp.Regexp
}

type _ValuePatterns []*_ValuePattern

// set sets the regexp for the name. If the name doesn't exist, it's added to
// the slice.
func (vps *_ValuePatterns) set(name string, vp *_ValuePattern) {
	var i, _ = vps.get(name)
	if i < 0 {
		*vps = append(*vps, vp)
		return
	}

	(*vps)[i] = vp
}

// get returns the index and the regexp of the name, if the name exists, or
// -1 and nil.
func (vps _ValuePatterns) get(name string) (int, *_ValuePattern) {
	for i, lvps := 0, len(vps); i < lvps; i++ {
		if vps[i].name == name {
			return i, vps[i]
		}
	}

	return -1, nil
}

// -------------------------

type _StringPair struct{ key, value string }

// TemplateValues is a slice of string key-value pairs.
type TemplateValues []_StringPair

// Set sets the value for the key. If the key doesn't exist, it's added to the
// slice.
func (tmplVs *TemplateValues) Set(key, value string) {
	var i, _ = tmplVs.get(key)
	if i < 0 {
		*tmplVs = append(*tmplVs, _StringPair{key, value})
		return
	}

	(*tmplVs)[i] = _StringPair{key, value}
}

// get returns the index and the value, if the key exists, or -1 and an empty
// string.
func (tmplVs TemplateValues) get(key string) (int, string) {
	for i := len(tmplVs) - 1; i >= 0; i-- {
		if tmplVs[i].key == key {
			return i, tmplVs[i].value
		}
	}

	return -1, ""
}

// Get returns the value of the key. If the key doesn't exist, it returns an
// empty string.
func (tmplVs TemplateValues) Get(key string) string {
	var _, v = tmplVs.get(key)
	return v
}

// --------------------------------------------------

type _TemplateSegment struct {
	staticStr    string         // Static segment of the template.
	valuePattern *_ValuePattern // Dynamic segment of the template.
}

// --------------------------------------------------

// Template represents the parsed template of the hosts and resources.
//
// A template may have three kinds of segments: static, regexp, and wildcard.
// The regexp segment must be between curly braces and its value name and
// pattern separated by ":". The wildcard segment can only have a value name.
// There can be only one wildcard segment in the template. The regexp and
// wildcard segments are the dynamic segments of the template. Dynamic
// segments' names are used as their value names.
//
// A template may have a name. The name comes at the beginning of the template,
// after the "$" sign and followed by ":". If there is a single dynamic segment
// in the template and the template doesn't have a name, the dynamic segment's
// name is used as the name of the template.
//
// If the regexp dynamic segment is repeated in the template, from the second
// repetition, its pattern may be omitted. When the regexp matches a string, its
// repetitions must get the same value, otherwise the match fails.
//
// The colon ":" in the template name and in the value name, as well as the
// curly braces "{" and "}" in the static segments, can be escaped with the
// backslash "\". The escaped colon ":" is included in the name, and the
// escaped curly braces "{" and "}" are included in the static segment. If the
// static segment at the beginning of the template must start with the "$"
// sign, and the template doesn't need a name, the "$" sign must be escaped too.
//
// Some examples of the template forms:
//
// 	$templateName:staticPart{valueName:pattern},
// 	$templateName:{valueName:pattern}staticpart,
// 	$templateName:{wildcardName}{valueName1:pattern1}{valueName2:pattern2},
// 	staticTemplate,
// 	{valueName:pattern},
// 	{wildcardSegmentName},
// 	{valueName:pattern}staticPart{wildcardName}{valueName},
// 	$templateName:staticPart1{wildCardName}staticPart2{valueName:pattern}
// 	$templateName:staticPart,
// 	$templateName:{valueName:pattern},
// 	$templateName\:1:{wildCard{Name}}staticPart{value{Name}:pattern},
// 	{valueName\:1:pattern}static\{Part\},
// 	\$staticPart:1{wildcardName}staticPart:2
type Template struct {
	name        string
	slices      []_TemplateSegment
	wildCardIdx int
}

// -----

// SetName sets the name of the template. The name becomes the name of the host
// or resource.
func (t *Template) SetName(name string) {
	t.name = name
}

// Name returns the name of the template.
func (t *Template) Name() string {
	return t.name
}

// ValueNames returns the value names given in the wildcard and regular
// expression segments.
func (t *Template) ValueNames() []string {
	var vns []string
	for _, slice := range t.slices {
		if slice.valuePattern != nil {
			vns = append(vns, slice.valuePattern.name)
		}
	}

	return vns
}

// HasValueName returns true if one of the dynamic segments of the
// template has one of the names.
func (t *Template) HasValueName(names ...string) bool {
	var (
		tvalueNames  = t.ValueNames()
		ltvalueNames = len(tvalueNames)
		lnames       = len(names)
	)

	for i := 0; i < lnames; i++ {
		for j := 0; j < ltvalueNames; j++ {
			if names[i] == tvalueNames[j] {
				return true
			}
		}
	}

	return false
}

// Content returns the escaped content of the template without a name.
// The pattern is omitted from repeated regexp segments starting from
// the second repetition.
func (t *Template) Content() string {
	var strb = strings.Builder{}

	if t.name == "" && len(t.slices) != 0 {
		if t.slices[0].staticStr != "" && t.slices[0].staticStr[0] == '$' {
			strb.WriteByte('\\')
		}
	}

	var vns map[string]bool

	for _, slice := range t.slices {
		if slice.staticStr != "" {
			var idx = 0
			for i, ch := range slice.staticStr {
				// '{' and '}' are escaped with a back slash '\'.
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

			if vns == nil {
				vns = make(map[string]bool)
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

// UnescapedContent returns the unescaped content of the template without a
// name. The pattern is omitted from repeated regexp segments starting from
// the second repetition.
func (t *Template) UnescapedContent() string {
	var strb = strings.Builder{}
	var vns map[string]bool

	for _, slice := range t.slices {
		if slice.staticStr != "" {
			strb.WriteString(slice.staticStr)
		} else {
			strb.WriteByte('{')
			strb.WriteString(slice.valuePattern.name)

			if vns == nil {
				vns = make(map[string]bool)
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

// IsStatic returns true if the template doesn't have any dynamic segments.
func (t *Template) IsStatic() bool {
	return len(t.slices) == 1 && t.slices[0].staticStr != ""
}

// IsWildcard returns true if the template doesn't have any static or regular
// expression segments.
func (t *Template) IsWildcard() bool {
	return len(t.slices) == 1 && t.wildCardIdx == 0
}

// HasPattern returns true if the template has any regular expression segments.
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

	if t.IsWildcard() {
		if anotherT.IsWildcard() {
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

	if anotherT.IsStatic() || anotherT.IsWildcard() {
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
// wildcard or regular expression segments, the method adds the slices of the
// argument string that match those segments to the values argument and returns
// it. The names of the wildcard and regular expression segments in the template
// are used as keys for the values. The values argument is returned as is when
// the template doesn't match.
func (t *Template) Match(
	str string,
	values TemplateValues,
) (bool, TemplateValues) {
	var ltslices = len(t.slices)
	var k = ltslices
	if t.wildCardIdx >= 0 {
		k = t.wildCardIdx
	}

	for i := 0; i < k; i++ {
		if t.slices[i].staticStr != "" {
			if strings.HasPrefix(str, t.slices[i].staticStr) {
				str = str[len(t.slices[i].staticStr):]
			} else {
				return false, values
			}
		} else {
			var vp = t.slices[i].valuePattern
			var idxs = vp.re.FindStringIndex(str)
			if idxs != nil {
				var v = str[:idxs[1]]
				if vi, vf := values.get(vp.name); vi >= 0 {
					if v != vf {
						return false, values
					}
				} else {
					if values == nil {
						values = make(TemplateValues, 0, 5)
					}

					values = append(values, _StringPair{vp.name, v})
				}

				str = str[idxs[1]:]
			} else {
				return false, values
			}
		}
	}

	for i := ltslices - 1; i > k; i-- {
		if t.slices[i].staticStr != "" {
			if strings.HasSuffix(str, t.slices[i].staticStr) {
				str = str[:len(str)-len(t.slices[i].staticStr)]
			} else {
				return false, values
			}
		} else {
			var vp = t.slices[i].valuePattern
			var idxs = vp.re.FindAllStringIndex(str, -1)
			if len(idxs) == 1 {
				var v = str[idxs[0][0]:]
				if vi, vf := values.get(vp.name); vi >= 0 {
					if v != vf {
						return false, values
					}
				} else {
					if values == nil {
						values = make(TemplateValues, 0, 5)
					}

					values = append(values, _StringPair{vp.name, v})
				}

				str = str[:idxs[0][0]]
			} else {
				return false, values
			}
		}
	}

	if len(str) > 0 {
		if t.wildCardIdx >= 0 {
			if values == nil {
				values = make(TemplateValues, 0, 5)
			}

			values = append(
				values,
				_StringPair{t.slices[t.wildCardIdx].valuePattern.name, str},
			)
		} else {
			return false, values
		}
	}

	return true, values
}

// Apply puts the values in the place of the wildcard segment and regular
// expression segments if they match. When ignoreMissing is true, Apply
// ignores the missing values for the segments instead of returning an error.
func (t *Template) Apply(values TemplateValues, ignoreMissing bool) (
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

		if vi, vf := values.get(slc.valuePattern.name); vi >= 0 {
			if slc.valuePattern.re != nil {
				var idxs = slc.valuePattern.re.FindStringIndex(vf)
				if idxs == nil || (idxs[0] != 0 && idxs[1] != len(vf)) {
					return "", newErr(
						"%w value for %q",
						ErrInvalidValue,
						slc.valuePattern.name,
					)
				}
			}

			strb.WriteString(vf)
		} else if ignoreMissing {
			continue
		} else {
			return "", newErr(
				"%w for %q",
				ErrMissingValue,
				slc.valuePattern.name,
			)
		}
	}

	return strb.String(), nil
}

// String returns the template's string. The pattern is omitted from
// repeated regexp segments starting from the second repetition.
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

// Clear clears the content and the name of the template.
func (t *Template) Clear() {
	t.name = ""
	t.slices = nil
	t.wildCardIdx = -1
}

// --------------------------------------------------

// templateNameAndContent divides the template string into its unescaped name
// and the escaped content. So the content can be parsed.
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

// staticSegment returns the static segment at the beginning of the template.
// If the template doesn't start with a static segment, it's returned as is.
func staticSegment(tmplStrSlc string) (
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

			// Escaped '{'.
			strb.WriteString(tmplStrSlc[idx : i-1])
			strb.WriteByte('{')
			idx = i + 1
		} else if ch == '}' {
			if pch != '\\' {
				err = newErr(
					"%w - unescaped curly brace '}' at index %d",
					ErrInvalidTemplate,
					i,
				)

				return
			}

			// Escaped '}'.
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

// dynamicSegment returns the dynamic segment's value name and pattern at the
// beginning of the template. If the template doesn't start with a dynamic
// segment, it's returned as is.
func dynamicSegment(tmplStrSlc string) (
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
					// If the previous character is a backslash "\", the current
					// character colon ":" is escaped. So, it's included in the
					// value name.
					if tmplStrSlc[i-1] == '\\' {
						strb.WriteString(tmplStrSlc[idx : i-1])
						strb.WriteByte(':')
						idx = i + 1
						continue
					}
				}

				if depth > 1 {
					err = newErr("%w - open curly brace", ErrInvalidTemplate)
					return
				}

				strb.WriteString(tmplStrSlc[idx:i])
				if strb.Len() == 0 {
					err = newErr("%w - empty value name", ErrInvalidTemplate)
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
					// Current curly brace "}" is not the end of the value name.
					continue
				}

				if strb.Len() > 0 {
					strb.WriteString(tmplStrSlc[idx:i])
					valueName = strb.String()
				} else {
					if i == idx {
						err = newErr(
							"%w - empty dynamic slice",
							ErrInvalidTemplate,
						)

						return
					}

					valueName = tmplStrSlc[idx:i]
				}

				leftTmplStrSlc = tmplStrSlc[i+1:]

				// Here we have a value name without a regular expression.
				return
			}
		}

		if sliceType == 1 {
			if ch == '\\' {
				i++
				// Backslack in a regular expression escapes any character.
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
					err = newErr("%w - empty regexp", ErrInvalidTemplate)
					return
				}

				pattern = tmplStrSlc[idx:i]
				leftTmplStrSlc = tmplStrSlc[i+1:]
				break
			}
		}
	}

	if depth > 0 {
		err = newErr("%w - incomplete dynamic segment", ErrInvalidTemplate)
	}

	return
}

// appendDynamicSegmentTo appends the value name and pattern to the list of
// dynamic segments. The slice valuePatterns contains the previously created
// _ValuePattern instances with value names as a key. If the value name is
// repeated, appendDynamicSegmentTo reuses the _ValuePattern instance instead
// of creating a new one.
//
// If the passed argument wildCardIdx is the index of the previously detected
// wild card, then it's returned as is. Otherwise, if the current dynamic
// segment is a wildcard, its index in the list is returned.
func appendDynamicSegmentTo(
	tss []_TemplateSegment,
	vName, pattern string,
	valuePatterns _ValuePatterns,
	wildcardIdx int,
) ([]_TemplateSegment, _ValuePatterns, int, error) {
	if vpi, vp := valuePatterns.get(vName); vpi >= 0 {
		if pattern != "" {
			if wildcardIdx >= 0 {
				pattern += "$"
			} else {
				pattern = "^" + pattern
			}

			if pattern != vp.re.String() {
				return tss, valuePatterns, -1, newErr(
					"%w for a value %q",
					ErrDifferentPattern,
					vName,
				)
			}
		}

		// If a value-pattern pair already exists, we don't have to create a
		// new one.
		tss = append(tss, _TemplateSegment{valuePattern: vp})
		return tss, valuePatterns, wildcardIdx, nil
	}

	if pattern == "" {
		if wildcardIdx >= 0 {
			var wc = tss[wildcardIdx]
			if vName == wc.valuePattern.name {
				return tss, valuePatterns, wildcardIdx, newErr(
					"%w %q",
					ErrRepeatedWildcardName,
					vName,
				)
			}

			return tss, valuePatterns, wildcardIdx, newErr(
				"%w %q",
				ErrAnotherWildcardName,
				vName,
			)
		}

		wildcardIdx = len(tss)
		tss = append(tss, _TemplateSegment{
			valuePattern: &_ValuePattern{name: vName},
		})

		// As the wildcard segment has been appended, existing value-patterns
		// must be modified so when they are reused, the template can match the
		// string from the end to the wildcard segment.
		for _, vp := range valuePatterns {
			var p = vp.re.String()
			p = p[1:] + "$"

			var re, err = regexp.Compile(p)
			if err != nil {
				return tss, valuePatterns, wildcardIdx, err
			}

			valuePatterns.set(vp.name, &_ValuePattern{vp.name, re})
		}

		return tss, valuePatterns, wildcardIdx, nil
	}

	if wildcardIdx >= 0 {
		pattern += "$"
	} else {
		pattern = "^" + pattern
	}

	var re, err = regexp.Compile(pattern)
	if err != nil {
		return tss, valuePatterns, wildcardIdx, err
	}

	var vp = &_ValuePattern{name: vName, re: re}
	tss = append(tss, _TemplateSegment{valuePattern: vp})
	valuePatterns.set(vName, vp)

	return tss, valuePatterns, wildcardIdx, nil
}

// parse parses the template string and returns the template segments and the
// index of the wildcard segment.
func parse(tmplStr string) (
	tmplSlcs []_TemplateSegment,
	wildcardIdx int,
	err error,
) {
	if tmplStr == "" {
		return nil, -1, newErr("%w", ErrInvalidTemplate)
	}

	var (
		tmplStrSlc = tmplStr
		tss        = []_TemplateSegment{}

		valuePatterns = make(_ValuePatterns, 0, 1)
	)

	wildcardIdx = -1

	for len(tmplStrSlc) > 0 {
		var staticStr string
		staticStr, tmplStrSlc, err = staticSegment(tmplStrSlc)
		if err != nil {
			return nil, -1, err
		}

		if staticStr != "" {
			tss = append(tss, _TemplateSegment{staticStr: staticStr})
		}

		if tmplStrSlc == "" {
			break
		}

		var vName, pattern string
		vName, pattern, tmplStrSlc, err = dynamicSegment(tmplStrSlc)
		if err != nil {
			return nil, -1, err
		}

		tss, valuePatterns, wildcardIdx, err = appendDynamicSegmentTo(
			tss,
			vName, pattern,
			valuePatterns,
			wildcardIdx,
		)

		if err != nil {
			return nil, -1, err
		}
	}

	tmplSlcs = make([]_TemplateSegment, len(tss))
	copy(tmplSlcs, tss)

	var lastIdx = len(tmplSlcs) - 1
	var vp = tmplSlcs[lastIdx].valuePattern
	if vp != nil && vp.re != nil && wildcardIdx < 0 {
		// The last segment is a value-pattern segment. So, its regexp must be
		// modified to match the end of the string.
		var reStr = vp.re.String() + "$"
		vp.re, err = regexp.Compile(reStr)
		if err != nil {
			return nil, -1, err
		}
	}

	return tmplSlcs, wildcardIdx, nil
}

// TryToParse tries to parse the passed template string, and if successful,
// returns the Template instance.
func TryToParse(tmplStr string) (*Template, error) {
	if tmplStr == "" {
		return nil, newErr(" %w - empty template", ErrInvalidTemplate)
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
		if tmpl.IsWildcard() {
			tmpl.name = tmpl.slices[0].valuePattern.name
		} else {
			var idx = -1
			for i, slc := range tmpl.slices {
				if slc.valuePattern != nil {
					if idx > -1 {
						// If there is a single dynamic segment in the template
						// and the template doesn't have a name, the dynamic
						// segment's name is used as the name of the template.
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
// it succeeds. Unlike TryToParse, Parse panics on an error.
func Parse(tmplStr string) *Template {
	var tmpl, err = TryToParse(tmplStr)
	if err != nil {
		panic(err)
	}

	return tmpl
}
