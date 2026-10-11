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

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path   string
		expect []string
	}{
		{"", []string{""}},
		{"simple", []string{"simple"}},
		{"users.name.first", []string{"users", "name", "first"}},
		{"users.first\\.name.0", []string{"users", "first\\.name", "0"}},
		{"items.[*].id", []string{"items", "[*]", "id"}},
		{"data.#[name.first=\"John\"].age", []string{"data", "#[name.first=\"John\"]", "age"}},
		{"friends.#(last=\"Murphy\").last", []string{"friends", "#(last=\"Murphy\")", "last"}},
		{"friends.#(last=\"Murphy\")#.last", []string{"friends", "#(last=\"Murphy\")#", "last"}},
		{"a.b\\.c.d", []string{"a", "b\\.c", "d"}},
	}

	for _, tc := range tests {
		got := SplitPath(tc.path)
		if len(got) != len(tc.expect) {
			t.Fatalf("SplitPath(%q) returned %d parts, want %d", tc.path, len(got), len(tc.expect))
		}
		for i := range got {
			if got[i] != tc.expect[i] {
				t.Fatalf("SplitPath(%q)[%d] = %q, want %q", tc.path, i, got[i], tc.expect[i])
			}
		}
	}
}

func TestJoinPath(t *testing.T) {
	if got := JoinPath(); got != "" {
		t.Fatalf("JoinPath() = %q, want empty string", got)
	}
	if got := JoinPath("user"); got != "user" {
		t.Fatalf("JoinPath(\"user\") = %q, want \"user\"", got)
	}
	if got := JoinPath("users", "profile", "name"); got != "users.profile.name" {
		t.Fatalf("JoinPath mismatch: got %q", got)
	}
	parts := []string{"items", "0", "sku"}
	if got := JoinPath(parts...); got != "items.0.sku" {
		t.Fatalf("JoinPath(parts...) mismatch: got %q", got)
	}
}

func TestIsComplexPath(t *testing.T) {
	complexPaths := []string{
		"users.#",
		"items.*.id",
		"list.[]",
		"users.#(age>30).name",
		"data.#[status=\"active\"].id",
	}
	for _, p := range complexPaths {
		if !IsComplexPath(p) {
			t.Fatalf("IsComplexPath(%q) = false, want true", p)
		}
	}

	simplePaths := []string{
		"users",
		"users.0.name",
		"a.b.c",
		"config.host\\.domain",
	}
	for _, p := range simplePaths {
		if IsComplexPath(p) {
			t.Fatalf("IsComplexPath(%q) = true, want false", p)
		}
	}
}

func TestKeysAndKeysBytes(t *testing.T) {
	json := `{"name":"John","age":30,"address":{"city":"Boston","state":"MA"},"tags":["a","b"]}`

	// Top-level keys
	keys := Keys(json, "")
	expectedTop := []string{"name", "age", "address", "tags"}
	if len(keys) != len(expectedTop) {
		t.Fatalf("Keys(json, \"\") length = %d, want %d", len(keys), len(expectedTop))
	}
	for i, k := range keys {
		if k != expectedTop[i] {
			t.Fatalf("Keys(json, \"\")[%d] = %q, want %q", i, k, expectedTop[i])
		}
	}

	// Nested object keys
	subKeys := Keys(json, "address")
	expectedSub := []string{"city", "state"}
	if len(subKeys) != len(expectedSub) {
		t.Fatalf("Keys(json, \"address\") length = %d, want %d", len(subKeys), len(expectedSub))
	}
	for i, k := range subKeys {
		if k != expectedSub[i] {
			t.Fatalf("Keys(json, \"address\")[%d] = %q, want %q", i, k, expectedSub[i])
		}
	}

	// Non-object target returns nil
	if k := Keys(json, "tags"); k != nil {
		t.Fatalf("Keys on array expected nil, got %v", k)
	}
	if k := Keys(json, "age"); k != nil {
		t.Fatalf("Keys on number expected nil, got %v", k)
	}

	// Missing path returns nil
	if k := Keys(json, "nonexistent"); k != nil {
		t.Fatalf("Keys on nonexistent path expected nil, got %v", k)
	}

	// KeysBytes parity
	bytesKeys := KeysBytes([]byte(json), "address")
	if len(bytesKeys) != len(expectedSub) {
		t.Fatalf("KeysBytes length mismatch: %d vs %d", len(bytesKeys), len(expectedSub))
	}
	for i, k := range bytesKeys {
		if k != expectedSub[i] {
			t.Fatalf("KeysBytes[%d] = %q, want %q", i, k, expectedSub[i])
		}
	}
}

func TestNestedArrayUpdate(t *testing.T) {
	json := `{"object1":{"object2":[{"nested_object1":{"nested_object2":[{"nested2_object1":1},{"nested2_object1":1}]}}]}}`
	path := "object1.object2.#.nested_object1.nested_object2.#.nested2_object1"
	out, err := Set(json, path, 2)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	expected := `{"object1":{"object2":[{"nested_object1":{"nested_object2":[{"nested2_object1":2},{"nested2_object1":2}]}}]}}`
	if out != expected {
		t.Fatalf("Set output mismatch:\n got: %s\nwant: %s", out, expected)
	}

	// Verify with gjson
	res := gjson.Get(out, path)
	if !res.Exists() {
		t.Fatalf("expected path to exist in output")
	}
	for _, val := range res.Array() {
		for _, nestedVal := range val.Array() {
			if nestedVal.Int() != 2 {
				t.Fatalf("expected nested value 2, got %v", nestedVal)
			}
		}
	}
}


