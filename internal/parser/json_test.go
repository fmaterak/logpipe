package parser

import "testing"

func TestJSONParser_ValidObject(t *testing.T) {
	p := NewJSON()
	entry := p.Parse(`{"level":"error","msg":"boom","code":42,"ratio":1.5,"ok":true}`)

	if entry.Fields == nil {
		t.Fatalf("expected fields to be populated")
	}
	cases := map[string]string{
		"level": "error",
		"msg":   "boom",
		"code":  "42",
		"ratio": "1.5",
		"ok":    "true",
	}
	for k, want := range cases {
		if got := entry.Fields[k]; got != want {
			t.Errorf("field %s: got %q want %q", k, got, want)
		}
	}
}

func TestJSONParser_FallsBackToRawForNonJSON(t *testing.T) {
	p := NewJSON()
	entry := p.Parse(`plain text not json`)
	if entry.Fields != nil {
		t.Fatalf("expected nil fields for non-JSON line, got %v", entry.Fields)
	}
	if entry.Raw != `plain text not json` {
		t.Errorf("raw not preserved: %q", entry.Raw)
	}
}

func TestJSONParser_NestedFieldStringified(t *testing.T) {
	p := NewJSON()
	entry := p.Parse(`{"meta":{"k":"v"}}`)
	got := entry.Fields["meta"]
	if got != `{"k":"v"}` {
		t.Errorf("nested object: got %q", got)
	}
}
