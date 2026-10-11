// Package sjson provides setting json values.
package sjson

import (
	jsongo "encoding/json"
	"strconv"
	"unsafe"

	"github.com/tidwall/gjson"
)

type errorType struct {
	msg string
}

func (err *errorType) Error() string {
	return err.msg
}

// Options represents additional options for the Set and Delete functions.
type Options struct {
	// Optimistic is a hint that the value likely exists which
	// allows for the sjson to perform a fast-track search and replace.
	Optimistic bool
	// ReplaceInPlace is a hint to replace the input json rather than
	// allocate a new json byte slice. When this field is specified
	// the input json will not longer be valid and it should not be used
	// In the case when the destination slice doesn't have enough free
	// bytes to replace the data in place, a new bytes slice will be
	// created under the hood.
	// The Optimistic flag must be set to true and the input must be a
	// byte slice in order to use this field.
	ReplaceInPlace bool
}

type pathResult struct {
	part  string // current key part
	gpart string // gjson get part
	path  string // remaining path
	force bool   // force a string key
	more  bool   // there is more path to parse
}

func isSimpleChar(ch byte) bool {
	switch ch {
	case '|', '#', '@', '*', '?':
		return false
	default:
		return true
	}
}

func parsePath(path string) (res pathResult, simple bool) {
	var r pathResult
	if len(path) > 0 && path[0] == ':' {
		r.force = true
		path = path[1:]
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			r.part = path[:i]
			r.gpart = path[:i]
			r.path = path[i+1:]
			r.more = true
			return r, true
		}
		if !isSimpleChar(path[i]) {
			return r, false
		}
		if path[i] == '\\' {
			// go into escape mode. this is a slower path that
			// strips off the escape character from the part.
			epart := []byte(path[:i])
			gpart := []byte(path[:i+1])
			i++
			if i < len(path) {
				epart = append(epart, path[i])
				gpart = append(gpart, path[i])
				i++
				for ; i < len(path); i++ {
					if path[i] == '\\' {
						gpart = append(gpart, '\\')
						i++
						if i < len(path) {
							epart = append(epart, path[i])
							gpart = append(gpart, path[i])
						}
						continue
					} else if path[i] == '.' {
						r.part = string(epart)
						r.gpart = string(gpart)
						r.path = path[i+1:]
						r.more = true
						return r, true
					} else if !isSimpleChar(path[i]) {
						return r, false
					}
					epart = append(epart, path[i])
					gpart = append(gpart, path[i])
				}
			}
			// append the last part
			r.part = string(epart)
			r.gpart = string(gpart)
			return r, true
		}
	}
	r.part = path
	r.gpart = path
	return r, true
}

func mustMarshalString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < ' ' || s[i] > 0x7f || s[i] == '"' || s[i] == '\\' {
			return true
		}
	}
	return false
}

// appendStringify makes a json string and appends to buf.
func appendStringify(buf []byte, s string) []byte {
	if mustMarshalString(s) {
		b, _ := jsongo.Marshal(s)
		return append(buf, b...)
	}
	buf = append(buf, '"')
	buf = append(buf, s...)
	buf = append(buf, '"')
	return buf
}

// appendBuild builds a json block from a json path.
func appendBuild(buf []byte, array bool, paths []pathResult, raw string,
	stringify bool) []byte {
	if !array {
		buf = appendStringify(buf, paths[0].part)
		buf = append(buf, ':')
	}
	if len(paths) > 1 {
		n, numeric := atoui(paths[1])
		if numeric || (!paths[1].force && paths[1].part == "-1") {
			buf = append(buf, '[')
			buf = appendRepeat(buf, "null,", n)
			buf = appendBuild(buf, true, paths[1:], raw, stringify)
			buf = append(buf, ']')
		} else {
			buf = append(buf, '{')
			buf = appendBuild(buf, false, paths[1:], raw, stringify)
			buf = append(buf, '}')
		}
	} else {
		if stringify {
			buf = appendStringify(buf, raw)
		} else {
			buf = append(buf, raw...)
		}
	}
	return buf
}

// atoui does a rip conversion of string -> unigned int.
func atoui(r pathResult) (n int, ok bool) {
	if r.force {
		return 0, false
	}
	for i := 0; i < len(r.part); i++ {
		if r.part[i] < '0' || r.part[i] > '9' {
			return 0, false
		}
		n = n*10 + int(r.part[i]-'0')
	}
	return n, true
}

// appendRepeat repeats string "n" times and appends to buf.
func appendRepeat(buf []byte, s string, n int) []byte {
	for i := 0; i < n; i++ {
		buf = append(buf, s...)
	}
	return buf
}

// trim does a rip trim
func trim(s string) string {
	for len(s) > 0 {
		if s[0] <= ' ' {
			s = s[1:]
			continue
		}
		break
	}
	for len(s) > 0 {
		if s[len(s)-1] <= ' ' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

// deleteTailItem deletes the previous key or comma.
func deleteTailItem(buf []byte) ([]byte, bool) {
loop:
	for i := len(buf) - 1; i >= 0; i-- {
		// look for either a ',',':','['
		switch buf[i] {
		case '[':
			return buf, true
		case ',':
			return buf[:i], false
		case ':':
			// delete tail string
			i--
			for ; i >= 0; i-- {
				if buf[i] == '"' {
					i--
					for ; i >= 0; i-- {
						if buf[i] == '"' {
							i--
							if i >= 0 && buf[i] == '\\' {
								i--
								continue
							}
							for ; i >= 0; i-- {
								// look for either a ',','{'
								switch buf[i] {
								case '{':
									return buf[:i+1], true
								case ',':
									return buf[:i], false
								}
							}
						}
					}
					break
				}
			}
			break loop
		}
	}
	return buf, false
}

var errNoChange = &errorType{"no change"}

func appendRawPaths(buf []byte, jstr string, paths []pathResult, raw string,
	stringify, del bool) ([]byte, error) {
	var err error
	var res gjson.Result
	var found bool
	if del {
		if paths[0].part == "-1" && !paths[0].force {
			res = gjson.Get(jstr, "#")
			if res.Int() > 0 {
				res = gjson.Get(jstr, strconv.FormatInt(int64(res.Int()-1), 10))
				found = true
			}
		}
	}
	if !found {
		res = gjson.Get(jstr, paths[0].gpart)
	}
	if res.Index > 0 {
		if len(paths) > 1 {
			buf = append(buf, jstr[:res.Index]...)
			buf, err = appendRawPaths(buf, res.Raw, paths[1:], raw,
				stringify, del)
			if err != nil {
				return nil, err
			}
			buf = append(buf, jstr[res.Index+len(res.Raw):]...)
			return buf, nil
		}
		buf = append(buf, jstr[:res.Index]...)
		var exidx int // additional forward stripping
		if del {
			var delNextComma bool
			buf, delNextComma = deleteTailItem(buf)
			if delNextComma {
				i, j := res.Index+len(res.Raw), 0
				for ; i < len(jstr); i, j = i+1, j+1 {
					if jstr[i] <= ' ' {
						continue
					}
					if jstr[i] == ',' {
						exidx = j + 1
					}
					break
				}
			}
		} else {
			if stringify {
				buf = appendStringify(buf, raw)
			} else {
				buf = append(buf, raw...)
			}
		}
		buf = append(buf, jstr[res.Index+len(res.Raw)+exidx:]...)
		return buf, nil
	}
	if del {
		return nil, errNoChange
	}
	n, numeric := atoui(paths[0])
	isempty := true
	for i := 0; i < len(jstr); i++ {
		if jstr[i] > ' ' {
			isempty = false
			break
		}
	}
	if isempty {
		if numeric {
			jstr = "[]"
		} else {
			jstr = "{}"
		}
	}
	jsres := gjson.Parse(jstr)
	if jsres.Type != gjson.JSON {
		if numeric {
			jstr = "[]"
		} else {
			jstr = "{}"
		}
		jsres = gjson.Parse(jstr)
	}
	var comma bool
	for i := 1; i < len(jsres.Raw); i++ {
		if jsres.Raw[i] <= ' ' {
			continue
		}
		if jsres.Raw[i] == '}' || jsres.Raw[i] == ']' {
			break
		}
		comma = true
		break
	}
	switch jsres.Raw[0] {
	default:
		return nil, &errorType{"json must be an object or array"}
	case '{':
		end := len(jsres.Raw) - 1
		for ; end > 0; end-- {
			if jsres.Raw[end] == '}' {
				break
			}
		}
		buf = append(buf, jsres.Raw[:end]...)
		if comma {
			buf = append(buf, ',')
		}
		buf = appendBuild(buf, false, paths, raw, stringify)
		buf = append(buf, '}')
		return buf, nil
	case '[':
		var appendit bool
		if !numeric {
			if paths[0].part == "-1" && !paths[0].force {
				appendit = true
			} else {
				return nil, &errorType{
					"cannot set array element for non-numeric key '" +
						paths[0].part + "'"}
			}
		}
		if appendit {
			njson := trim(jsres.Raw)
			if njson[len(njson)-1] == ']' {
				njson = njson[:len(njson)-1]
			}
			buf = append(buf, njson...)
			if comma {
				buf = append(buf, ',')
			}

			buf = appendBuild(buf, true, paths, raw, stringify)
			buf = append(buf, ']')
			return buf, nil
		}
		buf = append(buf, '[')
		ress := jsres.Array()
		for i := 0; i < len(ress); i++ {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, ress[i].Raw...)
		}
		if len(ress) == 0 {
			buf = appendRepeat(buf, "null,", n-len(ress))
		} else {
			buf = appendRepeat(buf, ",null", n-len(ress))
			if comma {
				buf = append(buf, ',')
			}
		}
		buf = appendBuild(buf, true, paths, raw, stringify)
		buf = append(buf, ']')
		return buf, nil
	}
}

func isOptimisticPath(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] < '.' || path[i] > 'z' {
			return false
		}
		if path[i] > '9' && path[i] < 'A' {
			return false
		}
		if path[i] > 'z' {
			return false
		}
	}
	return true
}

// Set sets a json value for the specified path.
// A path is in dot syntax, such as "name.last" or "age".
// This function expects that the json is well-formed, and does not validate.
// Invalid json will not panic, but it may return back unexpected results.
// An error is returned if the path is not valid.
//
// A path is a series of keys separated by a dot.
//
//  {
//    "name": {"first": "Tom", "last": "Anderson"},
//    "age":37,
//    "children": ["Sara","Alex","Jack"],
//    "friends": [
//      {"first": "James", "last": "Murphy"},
//      {"first": "Roger", "last": "Craig"}
//    ]
//  }
//  "name.last"          >> "Anderson"
//  "age"                >> 37
//  "children.1"         >> "Alex"
//
func Set(json, path string, value interface{}) (string, error) {
	return SetOptions(json, path, value, nil)
}

// SetBytes sets a json value for the specified path.
// If working with bytes, this method preferred over
// Set(string(data), path, value)
func SetBytes(json []byte, path string, value interface{}) ([]byte, error) {
	return SetBytesOptions(json, path, value, nil)
}

// SetRaw sets a raw json value for the specified path.
// This function works the same as Set except that the value is set as a
// raw block of json. This allows for setting premarshalled json objects.
func SetRaw(json, path, value string) (string, error) {
	return SetRawOptions(json, path, value, nil)
}

// SetRawOptions sets a raw json value for the specified path with options.
// This furnction works the same as SetOptions except that the value is set
// as a raw block of json. This allows for setting premarshalled json objects.
func SetRawOptions(json, path, value string, opts *Options) (string, error) {
	var optimistic bool
	if opts != nil {
		optimistic = opts.Optimistic
	}
	res, err := set(json, path, value, false, false, optimistic, false)
	if err == errNoChange {
		return json, nil
	}
	return string(res), err
}

// SetRawBytes sets a raw json value for the specified path.
// If working with bytes, this method preferred over
// SetRaw(string(data), path, value)
func SetRawBytes(json []byte, path string, value []byte) ([]byte, error) {
	return SetRawBytesOptions(json, path, value, nil)
}

type dtype struct{}

// Delete deletes a value from json for the specified path.
func Delete(json, path string) (string, error) {
	return Set(json, path, dtype{})
}

// DeleteOptions deletes a value from json for the specified path, with the given options.
func DeleteOptions(json, path string, options *Options) (string, error) {
	return SetOptions(json, path, dtype{}, options)
}

// DeleteBytes deletes a value from json for the specified path.
func DeleteBytes(json []byte, path string) ([]byte, error) {
	return SetBytes(json, path, dtype{})
}

// DeleteBytesOptions deletes a value from json for the specified path, with the given options.
func DeleteBytesOptions(json []byte, path string, options *Options) ([]byte, error) {
	return SetBytesOptions(json, path, dtype{}, options)
}

type stringHeader struct {
	data unsafe.Pointer
	len  int
}

type sliceHeader struct {
	data unsafe.Pointer
	len  int
	cap  int
}

func set(jstr, path, raw string,
	stringify, del, optimistic, inplace bool) ([]byte, error) {
	if path == "" {
		trimmed := trim(jstr)
		if len(trimmed) == 0 || trimmed[0] == '{' {
			paths := []pathResult{{part: "", gpart: ""}}
			njson, err := appendRawPaths(nil, jstr, paths, raw, stringify, del)
			if err != nil {
				return []byte(jstr), err
			}
			return njson, nil
		}
		return []byte(jstr), &errorType{"path cannot be empty"}
	}
	if !del && optimistic && isOptimisticPath(path) {
		res := gjson.Get(jstr, path)
		if res.Exists() && res.Index > 0 {
			sz := len(jstr) - len(res.Raw) + len(raw)
			if stringify {
				sz += 2
			}
			if inplace && sz <= len(jstr) {
				if !stringify || !mustMarshalString(raw) {
					jsonh := *(*stringHeader)(unsafe.Pointer(&jstr))
					jsonbh := sliceHeader{
						data: jsonh.data, len: jsonh.len, cap: jsonh.len}
					jbytes := *(*[]byte)(unsafe.Pointer(&jsonbh))
					if stringify {
						jbytes[res.Index] = '"'
						copy(jbytes[res.Index+1:], []byte(raw))
						jbytes[res.Index+1+len(raw)] = '"'
						copy(jbytes[res.Index+1+len(raw)+1:],
							jbytes[res.Index+len(res.Raw):])
					} else {
						copy(jbytes[res.Index:], []byte(raw))
						copy(jbytes[res.Index+len(raw):],
							jbytes[res.Index+len(res.Raw):])
					}
					return jbytes[:sz], nil
				}
				return []byte(jstr), nil
			}
			buf := make([]byte, 0, sz)
			buf = append(buf, jstr[:res.Index]...)
			if stringify {
				buf = appendStringify(buf, raw)
			} else {
				buf = append(buf, raw...)
			}
			buf = append(buf, jstr[res.Index+len(res.Raw):]...)
			return buf, nil
		}
	}
	var paths []pathResult
	r, simple := parsePath(path)
	if simple {
		paths = append(paths, r)
		for r.more {
			r, simple = parsePath(r.path)
			if !simple {
				break
			}
			paths = append(paths, r)
		}
	}
	if !simple {
		if del {
			return []byte(jstr),
				&errorType{"cannot delete value from a complex path"}
		}
		return setComplexPath(jstr, path, raw, stringify)
	}
	njson, err := appendRawPaths(nil, jstr, paths, raw, stringify, del)
	if err != nil {
		return []byte(jstr), err
	}
	return njson, nil
}

// SplitPath splits a path into its traversal components, handling escaped dots
// and complex query expressions.
func SplitPath(path string) []string {
	if len(path) == 0 {
		return []string{""}
	}
	var parts []string
	var start int
	var parenDepth int
	var inQuote bool
	var escaped bool

	for i := 0; i < len(path); i++ {
		ch := path[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote {
			if ch == '(' || ch == '[' {
				parenDepth++
			} else if ch == ')' || ch == ']' {
				if parenDepth > 0 {
					parenDepth--
				}
			} else if ch == '.' && parenDepth == 0 {
				parts = append(parts, path[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, path[start:])
	return parts
}

// JoinPath joins traversal components into a dot-separated path string.
func JoinPath(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	var buf []byte
	for i, part := range parts {
		if i > 0 {
			buf = append(buf, '.')
		}
		buf = append(buf, part...)
	}
	return string(buf)
}

// IsComplexPath reports whether the path contains query, array traversal, or
// wildcard selectors.
func IsComplexPath(path string) bool {
	parts := SplitPath(path)
	for _, part := range parts {
		if isComplexSelector(part) {
			return true
		}
	}
	return false
}

// Keys returns the object keys at the specified path.
// If path is empty, it returns the top-level keys of the JSON object.
// Returns nil if the target path does not exist or is not a JSON object.
func Keys(json, path string) []string {
	var res gjson.Result
	if path == "" {
		res = gjson.Parse(json)
	} else {
		res = gjson.Get(json, path)
	}
	if !res.Exists() || !res.IsObject() {
		return nil
	}
	var keys []string
	res.ForEach(func(key, _ gjson.Result) bool {
		keys = append(keys, key.String())
		return true
	})
	return keys
}

// KeysBytes returns the object keys at the specified path from a byte slice.
// If path is empty, it returns the top-level keys of the JSON object.
// Returns nil if the target path does not exist or is not a JSON object.
func KeysBytes(json []byte, path string) []string {
	jstr := *(*string)(unsafe.Pointer(&json))
	return Keys(jstr, path)
}

func isComplexSelector(part string) bool {
	if part == "#" || part == "*" || part == "[]" {
		return true
	}
	if len(part) >= 3 && part[0] == '#' && (part[1] == '(' || part[1] == '[') {
		return true
	}
	return false
}

func inBounds(length, start, end int) bool {
	return start >= 0 && end >= start && end <= length
}

func safeReplace(s string, start, end int, replacement string) (string, bool) {
	if !inBounds(len(s), start, end) {
		return s, false
	}
	var buf []byte
	buf = append(buf, s[:start]...)
	buf = append(buf, replacement...)
	buf = append(buf, s[end:]...)
	return string(buf), true
}

func unescapePart(s string) string {
	var buf []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			buf = append(buf, s[i])
			continue
		}
		buf = append(buf, s[i])
	}
	return string(buf)
}

func mutatePath(jstr string, comps []string, raw string, stringify, del bool) (string, error) {
	if len(comps) == 0 {
		return jstr, nil
	}
	cidx := -1
	for i, comp := range comps {
		if isComplexSelector(comp) {
			cidx = i
			break
		}
	}
	if cidx == -1 {
		paths := make([]pathResult, len(comps))
		for i, comp := range comps {
			part := comp
			force := false
			if len(part) > 0 && part[0] == ':' {
				force = true
				part = part[1:]
			}
			paths[i] = pathResult{
				part:  unescapePart(part),
				gpart: comp,
				force: force,
			}
		}
		res, err := appendRawPaths(nil, jstr, paths, raw, stringify, del)
		if err != nil {
			return jstr, err
		}
		return string(res), nil
	}
	if cidx > 0 {
		prefix := JoinPath(comps[:cidx]...)
		target := gjson.Get(jstr, prefix)
		if !target.Exists() {
			return jstr, errNoChange
		}
		if !inBounds(len(jstr), target.Index, target.Index+len(target.Raw)) ||
			jstr[target.Index:target.Index+len(target.Raw)] != target.Raw {
			return jstr, errNoChange
		}
		newTarget, err := mutateComplex(target.Raw, comps[cidx:], raw, stringify, del)
		if err != nil {
			return jstr, err
		}
		safeRep, ok := safeReplace(jstr, target.Index, target.Index+len(target.Raw), newTarget)
		if !ok {
			return jstr, errNoChange
		}
		return safeRep, nil
	}
	return mutateComplex(jstr, comps, raw, stringify, del)
}

func mutateComplex(arrJSON string, comps []string, raw string, stringify, del bool) (string, error) {
	trimmed := trim(arrJSON)
	if len(trimmed) < 2 || trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' {
		return arrJSON, &errorType{"complex path selector requires JSON array"}
	}
	sel := comps[0]
	suffix := comps[1:]

	arrRes := gjson.Parse(arrJSON)
	if !arrRes.Exists() || !arrRes.IsArray() {
		return arrJSON, &errorType{"invalid JSON array"}
	}
	elements := arrRes.Array()

	if sel == "#" || sel == "*" || sel == "[]" {
		newElements := make([]string, 0, len(elements))
		for _, el := range elements {
			if len(suffix) == 0 {
				if del {
					continue
				}
				if stringify {
					newElements = append(newElements, string(appendStringify(nil, raw)))
				} else {
					newElements = append(newElements, raw)
				}
			} else {
				newEl, err := mutatePath(el.Raw, suffix, raw, stringify, del)
				if err != nil {
					return arrJSON, err
				}
				newElements = append(newElements, newEl)
			}
		}
		var buf []byte
		buf = append(buf, '[')
		for i, el := range newElements {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, el...)
		}
		buf = append(buf, ']')
		return string(buf), nil
	}

	if len(sel) >= 3 && sel[0] == '#' && (sel[1] == '(' || sel[1] == '[') {
		isAll := sel[len(sel)-1] == '#'
		if isAll {
			query := sel[:len(sel)-1]
			newElements := make([]string, len(elements))
			for i, el := range elements {
				testArr := "[" + el.Raw + "]"
				match := gjson.Get(testArr, query)
				if match.Exists() {
					if len(suffix) == 0 {
						if del {
							continue
						}
						if stringify {
							newElements[i] = string(appendStringify(nil, raw))
						} else {
							newElements[i] = raw
						}
					} else {
						newEl, err := mutatePath(el.Raw, suffix, raw, stringify, del)
						if err != nil {
							return arrJSON, err
						}
						newElements[i] = newEl
					}
				} else {
					newElements[i] = el.Raw
				}
			}
			var buf []byte
			buf = append(buf, '[')
			for i, el := range newElements {
				if i > 0 {
					buf = append(buf, ',')
				}
				buf = append(buf, el...)
			}
			buf = append(buf, ']')
			return string(buf), nil
		}

		match := gjson.Get(arrJSON, sel)
		if !match.Exists() {
			return arrJSON, errNoChange
		}
		if !inBounds(len(arrJSON), match.Index, match.Index+len(match.Raw)) ||
			arrJSON[match.Index:match.Index+len(match.Raw)] != match.Raw {
			return arrJSON, errNoChange
		}
		var newEl string
		if len(suffix) == 0 {
			if stringify {
				newEl = string(appendStringify(nil, raw))
			} else {
				newEl = raw
			}
		} else {
			var err error
			newEl, err = mutatePath(match.Raw, suffix, raw, stringify, del)
			if err != nil {
				return arrJSON, err
			}
		}
		safeRep, ok := safeReplace(arrJSON, match.Index, match.Index+len(match.Raw), newEl)
		if !ok {
			return arrJSON, errNoChange
		}
		return safeRep, nil
	}

	return arrJSON, &errorType{"unsupported complex path selector: " + sel}
}

func setComplexPath(jstr, path, raw string, stringify bool) ([]byte, error) {
	comps := SplitPath(path)
	out, err := mutatePath(jstr, comps, raw, stringify, false)
	if err != nil {
		if err == errNoChange {
			return []byte(jstr), errNoChange
		}
		return []byte(jstr), err
	}
	return []byte(out), nil
}

// SetOptions sets a json value for the specified path with options.
// A path is in dot syntax, such as "name.last" or "age".
// This function expects that the json is well-formed, and does not validate.
// Invalid json will not panic, but it may return back unexpected results.
// An error is returned if the path is not valid.
func SetOptions(json, path string, value interface{},
	opts *Options) (string, error) {
	if opts != nil {
		if opts.ReplaceInPlace {
			// it's not safe to replace bytes in-place for strings
			// copy the Options and set options.ReplaceInPlace to false.
			nopts := *opts
			opts = &nopts
			opts.ReplaceInPlace = false
		}
	}
	jsonh := *(*stringHeader)(unsafe.Pointer(&json))
	jsonbh := sliceHeader{data: jsonh.data, len: jsonh.len, cap: jsonh.len}
	jsonb := *(*[]byte)(unsafe.Pointer(&jsonbh))
	res, err := SetBytesOptions(jsonb, path, value, opts)
	return string(res), err
}

// SetBytesOptions sets a json value for the specified path with options.
// If working with bytes, this method preferred over
// SetOptions(string(data), path, value)
func SetBytesOptions(json []byte, path string, value interface{},
	opts *Options) ([]byte, error) {
	var optimistic, inplace bool
	if opts != nil {
		optimistic = opts.Optimistic
		inplace = opts.ReplaceInPlace
	}
	jstr := *(*string)(unsafe.Pointer(&json))
	var res []byte
	var err error
	switch v := value.(type) {
	default:
		b, merr := jsongo.Marshal(value)
		if merr != nil {
			return nil, merr
		}
		raw := *(*string)(unsafe.Pointer(&b))
		res, err = set(jstr, path, raw, false, false, optimistic, inplace)
	case dtype:
		res, err = set(jstr, path, "", false, true, optimistic, inplace)
	case string:
		res, err = set(jstr, path, v, true, false, optimistic, inplace)
	case []byte:
		raw := *(*string)(unsafe.Pointer(&v))
		res, err = set(jstr, path, raw, true, false, optimistic, inplace)
	case bool:
		if v {
			res, err = set(jstr, path, "true", false, false, optimistic, inplace)
		} else {
			res, err = set(jstr, path, "false", false, false, optimistic, inplace)
		}
	case int8:
		res, err = set(jstr, path, strconv.FormatInt(int64(v), 10),
			false, false, optimistic, inplace)
	case int16:
		res, err = set(jstr, path, strconv.FormatInt(int64(v), 10),
			false, false, optimistic, inplace)
	case int32:
		res, err = set(jstr, path, strconv.FormatInt(int64(v), 10),
			false, false, optimistic, inplace)
	case int64:
		res, err = set(jstr, path, strconv.FormatInt(int64(v), 10),
			false, false, optimistic, inplace)
	case uint8:
		res, err = set(jstr, path, strconv.FormatUint(uint64(v), 10),
			false, false, optimistic, inplace)
	case uint16:
		res, err = set(jstr, path, strconv.FormatUint(uint64(v), 10),
			false, false, optimistic, inplace)
	case uint32:
		res, err = set(jstr, path, strconv.FormatUint(uint64(v), 10),
			false, false, optimistic, inplace)
	case uint64:
		res, err = set(jstr, path, strconv.FormatUint(uint64(v), 10),
			false, false, optimistic, inplace)
	case float32:
		res, err = set(jstr, path, strconv.FormatFloat(float64(v), 'f', -1, 64),
			false, false, optimistic, inplace)
	case float64:
		res, err = set(jstr, path, strconv.FormatFloat(float64(v), 'f', -1, 64),
			false, false, optimistic, inplace)
	}
	if err == errNoChange {
		return json, nil
	}
	return res, err
}

// SetRawBytesOptions sets a raw json value for the specified path with options.
// If working with bytes, this method preferred over
// SetRawOptions(string(data), path, value, opts)
func SetRawBytesOptions(json []byte, path string, value []byte,
	opts *Options) ([]byte, error) {
	jstr := *(*string)(unsafe.Pointer(&json))
	vstr := *(*string)(unsafe.Pointer(&value))
	var optimistic, inplace bool
	if opts != nil {
		optimistic = opts.Optimistic
		inplace = opts.ReplaceInPlace
	}
	res, err := set(jstr, path, vstr, false, false, optimistic, inplace)
	if err == errNoChange {
		return json, nil
	}
	return res, err
}
