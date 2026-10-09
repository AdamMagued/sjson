package sjson

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
)

const (
	setRaw    = 1
	setBool   = 2
	setInt    = 3
	setFloat  = 4
	setString = 5
	setDelete = 6
)

func sortJSON(json string) string {
	opts := pretty.Options{SortKeys: true}
	return string(pretty.Ugly(pretty.PrettyOptions([]byte(json), &opts)))
}

func testRaw(t *testing.T, kind int, expect, json, path string, value interface{}) {
	t.Helper()
	expect = sortJSON(expect)
	var json2 string
	var err error
	switch kind {
	default:
		json2, err = Set(json, path, value)
	case setRaw:
		json2, err = SetRaw(json, path, value.(string))
	case setDelete:
		json2, err = Delete(json, path)
	}

	if err != nil {
		t.Fatal(err)
	}
	json2 = sortJSON(json2)
	if json2 != expect {
		t.Fatalf("expected '%v', got '%v'", expect, json2)
	}
	var json3 []byte
	switch kind {
	default:
		json3, err = SetBytes([]byte(json), path, value)
	case setRaw:
		json3, err = SetRawBytes([]byte(json), path, []byte(value.(string)))
	case setDelete:
		json3, err = DeleteBytes([]byte(json), path)
	}
	json3 = []byte(sortJSON(string(json3)))
	if err != nil {
		t.Fatal(err)
	} else if string(json3) != expect {
		t.Fatalf("expected '%v', got '%v'", expect, string(json3))
	}
}
func TestBasic(t *testing.T) {
	testRaw(t, setRaw, `[{"hiw":"planet","hi":"world"}]`, `[{"hi":"world"}]`, "0.hiw", `"planet"`)
	testRaw(t, setRaw, `[true]`, ``, "0", `true`)
	testRaw(t, setRaw, `[null,true]`, ``, "1", `true`)
	testRaw(t, setRaw, `[1,null,true]`, `[1]`, "2", `true`)
	testRaw(t, setRaw, `[1,true,false]`, `[1,null,false]`, "1", `true`)
	testRaw(t, setRaw,
		`[1,{"hello":"when","this":[0,null,2]},false]`,
		`[1,{"hello":"when","this":[0,1,2]},false]`,
		"1.this.1", `null`)
	testRaw(t, setRaw,
		`{"a":1,"b":{"hello":"when","this":[0,null,2]},"c":false}`,
		`{"a":1,"b":{"hello":"when","this":[0,1,2]},"c":false}`,
		"b.this.1", `null`)
	testRaw(t, setRaw,
		`{"a":1,"b":{"hello":"when","this":[0,null,2,null,4]},"c":false}`,
		`{"a":1,"b":{"hello":"when","this":[0,null,2]},"c":false}`,
		"b.this.4", `4`)
	testRaw(t, setRaw,
		`{"b":{"this":[null,null,null,null,4]}}`,
		``,
		"b.this.4", `4`)
	testRaw(t, setRaw,
		`[null,{"this":[null,null,null,null,4]}]`,
		``,
		"1.this.4", `4`)
	testRaw(t, setRaw,
		`{"1":{"this":[null,null,null,null,4]}}`,
		``,
		":1.this.4", `4`)
	testRaw(t, setRaw,
		`{":1":{"this":[null,null,null,null,4]}}`,
		``,
		"\\:1.this.4", `4`)
	testRaw(t, setRaw,
		`{":\\1":{"this":[null,null,null,null,{".HI":4}]}}`,
		``,
		"\\:\\\\1.this.4.\\.HI", `4`)
	testRaw(t, setRaw,
		`{"app.token":"cde"}`,
		`{"app.token":"abc"}`,
		"app\\.token", `"cde"`)
	testRaw(t, setRaw,
		`{"b":{"this":{"😇":""}}}`,
		``,
		"b.this.😇", `""`)
	testRaw(t, setRaw,
		`[ 1,2  ,3]`,
		`  [ 1,2  ] `,
		"-1", `3`)
	testRaw(t, setInt, `[1234]`, ``, `0`, int64(1234))
	testRaw(t, setFloat, `[1234.5]`, ``, `0`, float64(1234.5))
	testRaw(t, setString, `["1234.5"]`, ``, `0`, "1234.5")
	testRaw(t, setBool, `[true]`, ``, `0`, true)
	testRaw(t, setBool, `[null]`, ``, `0`, nil)
	testRaw(t, setString, `{"arr":[1]}`, ``, `arr.-1`, 1)
	testRaw(t, setString, `{"a":"\\"}`, ``, `a`, "\\")
	testRaw(t, setString, `{"a":"C:\\Windows\\System32"}`, ``, `a`, `C:\Windows\System32`)
}

func TestDelete(t *testing.T) {
	testRaw(t, setDelete, `[456]`, `[123,456]`, `0`, nil)
	testRaw(t, setDelete, `[123,789]`, `[123,456,789]`, `1`, nil)
	testRaw(t, setDelete, `[123,456]`, `[123,456,789]`, `-1`, nil)
	testRaw(t, setDelete, `{"a":[123,456]}`, `{"a":[123,456,789]}`, `a.-1`, nil)
	testRaw(t, setDelete, `{"and":"another"}`, `{"this":"that","and":"another"}`, `this`, nil)
	testRaw(t, setDelete, `{"this":"that"}`, `{"this":"that","and":"another"}`, `and`, nil)
	testRaw(t, setDelete, `{}`, `{"and":"another"}`, `and`, nil)
	testRaw(t, setDelete, `{"1":"2"}`, `{"1":"2"}`, `3`, nil)
}

// TestRandomData is a fuzzing test that throws random data at SetRaw
// function looking for panics.
func TestRandomData(t *testing.T) {
	var lstr string
	defer func() {
		if v := recover(); v != nil {
			println("'" + hex.EncodeToString([]byte(lstr)) + "'")
			println("'" + lstr + "'")
			panic(v)
		}
	}()
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 200)
	for i := 0; i < 2000000; i++ {
		n, err := rand.Read(b[:rand.Int()%len(b)])
		if err != nil {
			t.Fatal(err)
		}
		lstr = string(b[:n])
		SetRaw(lstr, "zzzz.zzzz.zzzz", "123")
	}
}

func TestDeleteIssue21(t *testing.T) {
	json := `{"country_code_from":"NZ","country_code_to":"SA","date_created":"2018-09-13T02:56:11.25783Z","date_updated":"2018-09-14T03:15:16.67356Z","disabled":false,"last_edited_by":"Developers","id":"a3e...bc454","merchant_id":"f2b...b91abf","signed_date":"2018-02-01T00:00:00Z","start_date":"2018-03-01T00:00:00Z","url":"https://www.google.com"}`
	res1 := gjson.Get(json, "date_updated")
	var err error
	json, err = Delete(json, "date_updated")
	if err != nil {
		t.Fatal(err)
	}
	res2 := gjson.Get(json, "date_updated")
	res3 := gjson.Get(json, "date_created")
	if !res1.Exists() || res2.Exists() || !res3.Exists() {
		t.Fatal("bad news")
	}

	// We change the number of characters in this to make the section of the string before the section that we want to delete a certain length

	//---------------------------
	lenBeforeToDeleteIs307AsBytes := `{"1":"","0":"012345678901234567890123456789012345678901234567890123456789012345678901234567","to_delete":"0","2":""}`

	expectedForLenBefore307AsBytes := `{"1":"","0":"012345678901234567890123456789012345678901234567890123456789012345678901234567","2":""}`
	//---------------------------

	//---------------------------
	lenBeforeToDeleteIs308AsBytes := `{"1":"","0":"0123456789012345678901234567890123456789012345678901234567890123456789012345678","to_delete":"0","2":""}`

	expectedForLenBefore308AsBytes := `{"1":"","0":"0123456789012345678901234567890123456789012345678901234567890123456789012345678","2":""}`
	//---------------------------

	//---------------------------
	lenBeforeToDeleteIs309AsBytes := `{"1":"","0":"01234567890123456789012345678901234567890123456789012345678901234567890123456","to_delete":"0","2":""}`

	expectedForLenBefore309AsBytes := `{"1":"","0":"01234567890123456789012345678901234567890123456789012345678901234567890123456","2":""}`
	//---------------------------

	var data = []struct {
		desc     string
		input    string
		expected string
	}{
		{
			desc:     "len before \"to_delete\"... = 307",
			input:    lenBeforeToDeleteIs307AsBytes,
			expected: expectedForLenBefore307AsBytes,
		},
		{
			desc:     "len before \"to_delete\"... = 308",
			input:    lenBeforeToDeleteIs308AsBytes,
			expected: expectedForLenBefore308AsBytes,
		},
		{
			desc:     "len before \"to_delete\"... = 309",
			input:    lenBeforeToDeleteIs309AsBytes,
			expected: expectedForLenBefore309AsBytes,
		},
	}

	for i, d := range data {
		result, err := Delete(d.input, "to_delete")

		if err != nil {
			t.Error(fmtErrorf(testError{
				unexpected: "error",
				desc:       d.desc,
				i:          i,
				lenInput:   len(d.input),
				input:      d.input,
				expected:   d.expected,
				result:     result,
			}))
		}
		if result != d.expected {
			t.Error(fmtErrorf(testError{
				unexpected: "result",
				desc:       d.desc,
				i:          i,
				lenInput:   len(d.input),
				input:      d.input,
				expected:   d.expected,
				result:     result,
			}))
		}
	}
}

type testError struct {
	unexpected string
	desc       string
	i          int
	lenInput   int
	input      interface{}
	expected   interface{}
	result     interface{}
}

func fmtErrorf(e testError) string {
	return fmt.Sprintf(
		"Unexpected %s:\n\t"+
			"for=%q\n\t"+
			"i=%d\n\t"+
			"len(input)=%d\n\t"+
			"input=%v\n\t"+
			"expected=%v\n\t"+
			"result=%v",
		e.unexpected, e.desc, e.i, e.lenInput, e.input, e.expected, e.result,
	)
}

func TestSetDotKeyIssue10(t *testing.T) {
	json := `{"app.token":"abc"}`
	json, _ = Set(json, `app\.token`, "cde")
	if json != `{"app.token":"cde"}` {
		t.Fatalf("expected '%v', got '%v'", `{"app.token":"cde"}`, json)
	}
}
func TestDeleteDotKeyIssue19(t *testing.T) {
	json := []byte(`{"data":{"key1":"value1","key2.something":"value2"}}`)
	json, _ = DeleteBytes(json, `data.key2\.something`)
	if string(json) != `{"data":{"key1":"value1"}}` {
		t.Fatalf("expected '%v', got '%v'", `{"data":{"key1":"value1"}}`, json)
	}
}

func TestIssue36(t *testing.T) {
	var json = `
	{
	    "size": 1000
    }
`
	var raw = `
	{
	    "sample": "hello"
	}
`
	_ = raw
	if true {
		json, _ = SetRaw(json, "aggs", raw)
	}
	if !gjson.Valid(json) {
		t.Fatal("invalid json")
	}
	res := gjson.Get(json, "aggs.sample").String()
	if res != "hello" {
		t.Fatal("unexpected result")
	}
}

var example = `
{
	"name": {"first": "Tom", "last": "Anderson"},
	"age":37,
	"children": ["Sara","Alex","Jack"],
	"fav.movie": "Deer Hunter",
	"friends": [
	  {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
	  {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
	  {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
	]
  }
  `

func TestIndex(t *testing.T) {
	path := `friends.#(last="Murphy").last`
	json, err := Set(example, path, "Johnson")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, "friends.#.last").String() != `["Johnson","Craig","Murphy"]` {
		t.Fatal("mismatch")
	}
}

func TestIndexes(t *testing.T) {
	path := `friends.#(last="Murphy")#.last`
	json, err := Set(example, path, "Johnson")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, "friends.#.last").String() != `["Johnson","Craig","Johnson"]` {
		t.Fatal("mismatch")
	}
}

func TestIssue61(t *testing.T) {
	json := `{
		"@context": {
		  "rdfs": "http://www.w3.org/2000/01/rdf-schema#",
		  "@vocab": "http://schema.org/",
		  "sh": "http://www.w3.org/ns/shacl#"
		}
	}`
	json1, _ := Set(json, "@context.@vocab", "newval")
	if gjson.Get(json1, "@context.@vocab").String() != "newval" {
		t.Fail()
	}
}

func TestEscape(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"", ""},
		{"simple", "simple"},
		{"first.name", "first\\.name"},
		{"a.b.c", "a\\.b\\.c"},
		{"user*name", "user\\*name"},
		{"user?name", "user\\?name"},
		{"order#1", "order\\#1"},
		{"@context", "\\@context"},
		{"a|b", "a\\|b"},
		{"path\\to", "path\\\\to"},
		{":id", "\\:id"},
		{"foo:bar", "foo:bar"},
		{"item[0]", "item\\[0\\]"},
		{"key{1}", "key\\{1\\}"},
		{"!#$%&'()*+,/:;<=>?@[\\]^`{|}~", "\\!\\#\\$\\%\\&\\'\\(\\)\\*\\+\\,\\/:\\;\\<\\=\\>\\?\\@\\[\\\\\\]\\^\\`\\{\\|\\}\\~"},
		{"user name", "user name"},
		{"user-name_1", "user-name_1"},
		{"café", "café"},
	}

	for _, tc := range tests {
		got := Escape(tc.input)
		if got != tc.expect {
			t.Fatalf("Escape(%q) = %q, expected %q", tc.input, got, tc.expect)
		}
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"", ""},
		{"simple", "simple"},
		{"first\\.name", "first.name"},
		{"a\\.b\\.c", "a.b.c"},
		{"user\\*name", "user*name"},
		{"user\\?name", "user?name"},
		{"order\\#1", "order#1"},
		{"\\@context", "@context"},
		{"a\\|b", "a|b"},
		{"path\\\\to", "path\\to"},
		{"\\:id", ":id"},
		{"foo:bar", "foo:bar"},
		{"trailing\\", "trailing"},
	}

	for _, tc := range tests {
		got := Unescape(tc.input)
		if got != tc.expect {
			t.Fatalf("Unescape(%q) = %q, expected %q", tc.input, got, tc.expect)
		}
	}

	// Roundtrip property: Unescape(Escape(s)) == s
	roundtrips := []string{
		"",
		"normal",
		"first.name",
		"a.b.c.d",
		":colon",
		"mid:colon",
		"user*name?",
		"order#42@test|pipe",
		"path\\to\\dir",
		"mixed.symbols*and?dots",
	}
	for _, s := range roundtrips {
		escaped := Escape(s)
		round := Unescape(escaped)
		if round != s {
			t.Fatalf("roundtrip mismatch for %q: escaped=%q, unescaped=%q", s, escaped, round)
		}
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		parts  []string
		expect string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"", ""}, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b", "c"}, "a.b.c"},
		{[]string{"a", "", "b"}, "a.b"},
		{[]string{"", "a", "b"}, "a.b"},
		{[]string{"a.", ".b"}, "a.b"},
		{[]string{"...a...", "...b..."}, "a.b"},
		{[]string{"users", Escape("first.name"), "last"}, "users.first\\.name.last"},
		{[]string{"users", "key\\.", "next"}, "users.key\\..next"},
	}

	for _, tc := range tests {
		got := JoinPath(tc.parts...)
		if got != tc.expect {
			t.Fatalf("JoinPath(%v) = %q, expected %q", tc.parts, got, tc.expect)
		}
	}
}

func TestBuildPath(t *testing.T) {
	tests := []struct {
		parts  []string
		expect string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"users", "first.name", "last"}, "users.first\\.name.last"},
		{[]string{"users", "user@domain.com", "profile"}, "users.user\\@domain\\.com.profile"},
		{[]string{"items", "0", "name*"}, "items.0.name\\*"},
		{[]string{"nested", "", "valid"}, "nested.valid"},
	}

	for _, tc := range tests {
		got := BuildPath(tc.parts...)
		if got != tc.expect {
			t.Fatalf("BuildPath(%v) = %q, expected %q", tc.parts, got, tc.expect)
		}
	}
}

func TestForceKey(t *testing.T) {
	if got := ForceKey("2313"); got != ":2313" {
		t.Fatalf("expected ':2313', got %q", got)
	}
	if got := ForceKey("0"); got != ":0" {
		t.Fatalf("expected ':0', got %q", got)
	}
	if got := ForceKey("key.with.dots"); got != ":key\\.with\\.dots" {
		t.Fatalf("expected ':key\\.with\\.dots', got %q", got)
	}

	// Verify setting an object key rather than an array index
	json, err := Set("{}", JoinPath("users", ForceKey("2313"), "name"), "Sara")
	if err != nil {
		t.Fatal(err)
	}
	expect := `{"users":{"2313":{"name":"Sara"}}}`
	if json != expect {
		t.Fatalf("expected %q, got %q", expect, json)
	}
	if gjson.Get(json, "users.2313.name").String() != "Sara" {
		t.Fatalf("gjson lookup mismatch")
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path   string
		expect []string
	}{
		{"", nil},
		{"simple", []string{"simple"}},
		{"users.name.first", []string{"users", "name", "first"}},
		{"users.first\\.name.0", []string{"users", "first\\.name", "0"}},
		{"fav\\.movie", []string{"fav\\.movie"}},
		{"app\\.token.id", []string{"app\\.token", "id"}},
		{"\\:1.this.4.\\.HI", []string{"\\:1", "this", "4", "\\.HI"}},
		{"path\\\\.to.file", []string{"path\\\\", "to", "file"}},
		{"a\\.b\\.c", []string{"a\\.b\\.c"}},
		{"items.[*].id", []string{"items", "[*]", "id"}},
		{"items.[0].id", []string{"items", "[0]", "id"}},
		{"items.[?].name", []string{"items", "[?]", "name"}},
		{"store.books.[*].title", []string{"store", "books", "[*]", "title"}},
		{"items.[0,1].name", []string{"items", "[0,1]", "name"}},
		{"users.[profile.age>30].id", []string{"users", "[profile.age>30]", "id"}},
		{"data.#[name.first=\"John\"].age", []string{"data", "#[name.first=\"John\"]", "age"}},
		{"friends.#(last=\"Murphy\").last", []string{"friends", "#(last=\"Murphy\")", "last"}},
		{"friends.#(last=\"Murphy\")#.last", []string{"friends", "#(last=\"Murphy\")#", "last"}},
		{"friends.#(first%\"D*\").last", []string{"friends", "#(first%\"D*\")", "last"}},
		{"friends.#(name=\"Jane.Doe\").age", []string{"friends", "#(name=\"Jane.Doe\")", "age"}},
		{"friends.#(nets.#(==\"fb\"))#.first", []string{"friends", "#(nets.#(==\"fb\"))#", "first"}},
		{":0.name", []string{":0", "name"}},
		{"children.@reverse.0", []string{"children", "@reverse", "0"}},
		{"{a,b}.c", []string{"{a,b}", "c"}},
	}

	for _, tc := range tests {
		got := SplitPath(tc.path)
		if len(got) != len(tc.expect) {
			t.Fatalf("SplitPath(%q) returned %d parts, expected %d (%v vs %v)",
				tc.path, len(got), len(tc.expect), got, tc.expect)
		}
		for i := range got {
			if got[i] != tc.expect[i] {
				t.Fatalf("SplitPath(%q)[%d] = %q, expected %q", tc.path, i, got[i], tc.expect[i])
			}
		}

		// Verify DecomposePath produces identical results
		decomposed := DecomposePath(tc.path)
		if len(decomposed) != len(tc.expect) {
			t.Fatalf("DecomposePath(%q) mismatch", tc.path)
		}
	}
}

func TestSplitPathUnescaped(t *testing.T) {
	tests := []struct {
		path   string
		expect []string
	}{
		{"", nil},
		{"users.first\\.name.0", []string{"users", "first.name", "0"}},
		{"accounts.user\\@domain\\.com.role", []string{"accounts", "user@domain.com", "role"}},
		{"items.key\\[0\\].name", []string{"items", "key[0]", "name"}},
	}

	for _, tc := range tests {
		got := SplitPathUnescaped(tc.path)
		if len(got) != len(tc.expect) {
			t.Fatalf("SplitPathUnescaped(%q) returned %d parts, expected %d",
				tc.path, len(got), len(tc.expect))
		}
		for i := range got {
			if got[i] != tc.expect[i] {
				t.Fatalf("SplitPathUnescaped(%q)[%d] = %q, expected %q",
					tc.path, i, got[i], tc.expect[i])
			}
		}
	}

	// Roundtrip: SplitPathUnescaped(BuildPath(parts...)) == parts
	inputParts := []string{"catalog", "product.1", "detail[spec]", "version#2"}
	built := BuildPath(inputParts...)
	unescaped := SplitPathUnescaped(built)
	if len(unescaped) != len(inputParts) {
		t.Fatalf("roundtrip length mismatch")
	}
	for i := range unescaped {
		if unescaped[i] != inputParts[i] {
			t.Fatalf("roundtrip mismatch at %d: got %q, expected %q", i, unescaped[i], inputParts[i])
		}
	}
}

func TestForEachPathAndTraversePath(t *testing.T) {
	path := "users.profile.emails.0"
	var visited []string
	ForEachPath(path, func(part string) bool {
		visited = append(visited, part)
		return true
	})
	expected := []string{"users", "profile", "emails", "0"}
	if len(visited) != len(expected) {
		t.Fatalf("ForEachPath length mismatch")
	}
	for i := range visited {
		if visited[i] != expected[i] {
			t.Fatalf("ForEachPath item mismatch at %d", i)
		}
	}

	// Early termination test
	var early []string
	ForEachPath(path, func(part string) bool {
		early = append(early, part)
		return len(early) < 2
	})
	if len(early) != 2 || early[0] != "users" || early[1] != "profile" {
		t.Fatalf("ForEachPath early termination failed: %v", early)
	}

	// TraversePath alias test
	var traverseVisited []string
	TraversePath(path, func(part string) bool {
		traverseVisited = append(traverseVisited, part)
		return true
	})
	if len(traverseVisited) != len(expected) {
		t.Fatalf("TraversePath length mismatch")
	}
}

func TestPathHelpersSetAndGet(t *testing.T) {
	var json string
	var err error

	// 1. Dynamic path with dot-containing key
	path1 := BuildPath("accounts", "user.1", "email")
	json, err = Set(json, path1, "user1@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, path1).String() != "user1@example.com" {
		t.Fatalf("expected email match")
	}

	// 2. Dynamic path with wildcard and special characters
	path2 := BuildPath("accounts", "user*admin?", "role")
	json, err = Set(json, path2, "superuser")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, path2).String() != "superuser" {
		t.Fatalf("expected role match")
	}

	// 3. Dynamic path with array append using JoinPath
	path3 := JoinPath("accounts", Escape("user.1"), "tags", "-1")
	json, err = Set(json, path3, "active")
	if err != nil {
		t.Fatal(err)
	}
	json, err = Set(json, path3, "verified")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, JoinPath("accounts", Escape("user.1"), "tags", "0")).String() != "active" {
		t.Fatalf("expected tag 0 match")
	}
	if gjson.Get(json, JoinPath("accounts", Escape("user.1"), "tags", "1")).String() != "verified" {
		t.Fatalf("expected tag 1 match")
	}

	// 4. Dynamic path with Delete
	deletePath := BuildPath("accounts", "user.1", "email")
	json, err = Delete(json, deletePath)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, deletePath).Exists() {
		t.Fatalf("expected email to be deleted")
	}

	// 5. Bytes version
	rawBytes := []byte(`{}`)
	rawBytes, err = SetBytes(rawBytes, BuildPath("meta", "@context"), "schema.org")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(rawBytes, "meta.\\@context").String() != "schema.org" {
		t.Fatalf("expected @context match")
	}

	// 6. Traverse path segments dynamically
	complexPath := "config.servers.0.hostname"
	segments := SplitPath(complexPath)
	rebuilt := JoinPath(segments...)
	if rebuilt != complexPath {
		t.Fatalf("JoinPath(SplitPath) mismatch: %q vs %q", rebuilt, complexPath)
	}
	json, err = Set("{}", rebuilt, "prod-app-01")
	if err != nil {
		t.Fatal(err)
	}
	if gjson.Get(json, complexPath).String() != "prod-app-01" {
		t.Fatalf("expected hostname match")
	}
}
