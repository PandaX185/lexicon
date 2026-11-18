package cli

import "testing"

func TestParseLanguagePair(t *testing.T) {
	tests := []struct {
		input string
		src   string
		tgt   string
		ok    bool
	}{
		{input: "en-de", src: "en", tgt: "de", ok: true},
		{input: " EN-DE ", src: "en", tgt: "de", ok: true},
		{input: "jp-de", src: "ja", tgt: "de", ok: true},
		{input: "en:zh-hant", src: "en", tgt: "zh-hant", ok: true},
		{input: "en>de", src: "en", tgt: "de", ok: true},
		{input: "en", ok: false},
		{input: "en-en", ok: false},
		{input: "english-de", ok: false},
		{input: "en-", ok: false},
	}
	for _, test := range tests {
		src, tgt, err := parseLanguagePair(test.input)
		if test.ok {
			if err != nil {
				t.Fatalf("parseLanguagePair(%q): %v", test.input, err)
			}
			if src != test.src || tgt != test.tgt {
				t.Fatalf("parseLanguagePair(%q) = %s, %s; want %s, %s", test.input, src, tgt, test.src, test.tgt)
			}
			continue
		}
		if err == nil {
			t.Fatalf("parseLanguagePair(%q) succeeded; want an error", test.input)
		}
	}
}

func TestPairArgument(t *testing.T) {
	if got := pairArgument("en", "de"); got != "en-de" {
		t.Fatalf("pairArgument(en, de) = %q", got)
	}
	if got := pairArgument("zh-hant", "en"); got != "zh-hant:en" {
		t.Fatalf("pairArgument(zh-hant, en) = %q", got)
	}
}
