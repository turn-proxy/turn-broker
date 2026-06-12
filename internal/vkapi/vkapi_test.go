package vkapi

import "testing"

func TestDecodeResponse(t *testing.T) {
	ok, err := decodeResponse[callsGetSettingsResult]("m", []byte(`{"response":{"settings":{"public_key":"PK"}}}`))
	if err != nil || ok.Settings.PublicKey != "PK" {
		t.Fatalf("success: got %+v err=%v", ok, err)
	}

	_, err = decodeResponse[callsGetSettingsResult]("m", []byte(`{"error":{"error_code":5,"error_msg":"auth"}}`))
	if ae, ok := AsAPIError(err); !ok || ae.ErrorCode != AuthFailed {
		t.Fatalf("nested error not typed: ae=%+v ok=%v err=%v", ae, ok, err)
	}

	if _, err := decodeResponse[callsGetSettingsResult]("m", []byte(`{}`)); err == nil {
		t.Fatal("empty body (no response, no error) must be an error")
	}
}

func TestAPIErrorFromBody(t *testing.T) {
	ae, ok := apiErrorFromBody([]byte(`{"error_code":1114,"error_msg":"VCHAT_DETAILED_ERROR : join_link.decode_error"}`))
	if !ok || ae.ErrorCode != 1114 {
		t.Fatalf("got %+v ok=%v, want code 1114", ae, ok)
	}
	if _, ok := apiErrorFromBody([]byte(`{"turn_server":{"urls":["x"]}}`)); ok {
		t.Fatal("a success body must not be read as an error")
	}
	if _, ok := apiErrorFromBody([]byte(`not json`)); ok {
		t.Fatal("invalid json must not be read as an error")
	}
}

func TestNormalizeFbDo(t *testing.T) {
	cases := map[string]string{
		"https://x.okcdn.ru/":       "https://x.okcdn.ru/fb.do",
		"https://x.okcdn.ru":        "https://x.okcdn.ru/fb.do",
		"https://x.okcdn.ru/fb.do":  "https://x.okcdn.ru/fb.do",
		"https://x.okcdn.ru/fb.do/": "https://x.okcdn.ru/fb.do",
	}
	for in, want := range cases {
		if got := normalizeFbDo(in); got != want {
			t.Errorf("normalizeFbDo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCookiesFromJSONFiltersDomains(t *testing.T) {
	raw := `[
		{"name":"a","value":"1","domain":".vk.com"},
		{"name":"b","value":"2","domain":".login.vk.com"},
		{"name":"c","value":"3","domain":".google.com"},
		{"name":"d","value":"4","domain":"vk.com"},
		{"name":"e","value":"5","domain":"vk.ru"}
	]`
	cookies, err := CookiesFromJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range cookies {
		got[c.Name] = c.Value
	}
	for name, want := range map[string]string{"a": "1", "b": "2", "d": "4"} {
		if got[name] != want {
			t.Errorf("cookie %q = %q, want %q", name, got[name], want)
		}
	}
	for _, bad := range []string{"c", "e"} {
		if _, ok := got[bad]; ok {
			t.Errorf("cookie %q should have been filtered out", bad)
		}
	}
}

func TestCookiesFromJSONPreservesDomain(t *testing.T) {
	raw := `[{"name":"b","value":"2","domain":".login.vk.com"}]`
	cookies, err := CookiesFromJSON([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 1 || cookies[0].Domain != ".login.vk.com" {
		t.Fatalf("got %+v", cookies)
	}
}

func TestCookiesFromJSONRejectsEmptyFilter(t *testing.T) {
	raw := `[{"name":"a","value":"1","domain":".google.com"}]`
	if _, err := CookiesFromJSON([]byte(raw)); err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractCallToken(t *testing.T) {
	tok, err := ExtractCallToken("https://vk.com/call/join/0RmWByF5_vpGekgfIwNLMNEi5mNdkxMlUqlEJ31HOlY")
	if err != nil || tok != "0RmWByF5_vpGekgfIwNLMNEi5mNdkxMlUqlEJ31HOlY" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	if tok, err := ExtractCallToken("https://vk.ru/call/join/abcDEF"); err != nil || tok != "abcDEF" {
		t.Fatalf("vk.ru: tok=%q err=%v", tok, err)
	}
	for _, bad := range []string{
		"http://vk.com/call/join/x",
		"https://evil.com/call/join/x",
		"https://vk.com/wall/123",
	} {
		if _, err := ExtractCallToken(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestParseExpiryFromUsername(t *testing.T) {
	v, err := parseExpiryFromUsername("1700000000:abcdef")
	if err != nil || v != 1700000000 {
		t.Fatalf("v=%d err=%v", v, err)
	}
	if _, err := parseExpiryFromUsername("noexpiry"); err == nil {
		t.Fatal("expected error")
	}
}
