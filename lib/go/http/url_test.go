// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TODO(rsc):
//	test URLUnescape
//	test URLEscape
//	test ParseURL

type URLTest struct {
	in        string
	out       *URL
	roundtrip string // expected result of reserializing the URL; empty means same as "in".
}

var urltests = []URLTest{
	// no path
	{
		"http://www.google.com",
		&URL{
			Raw:          "http://www.google.com",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
		},
		"",
	},
	// path
	{
		"http://www.google.com/",
		&URL{
			Raw:          "http://www.google.com/",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/",
			Path:         "/",
		},
		"",
	},
	// path with hex escaping
	{
		"http://www.google.com/file%20one%26two",
		&URL{
			Raw:          "http://www.google.com/file%20one%26two",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/file%20one%26two",
			Path:         "/file one&two",
		},
		"http://www.google.com/file%20one&two",
	},
	// user
	{
		"ftp://webmaster@www.google.com/",
		&URL{
			Raw:          "ftp://webmaster@www.google.com/",
			Scheme:       "ftp",
			RawAuthority: "webmaster@www.google.com",
			RawUserinfo:  "webmaster",
			Host:         "www.google.com",
			RawPath:      "/",
			Path:         "/",
		},
		"",
	},
	// escape sequence in username
	{
		"ftp://john%20doe@www.google.com/",
		&URL{
			Raw:          "ftp://john%20doe@www.google.com/",
			Scheme:       "ftp",
			RawAuthority: "john%20doe@www.google.com",
			RawUserinfo:  "john%20doe",
			Host:         "www.google.com",
			RawPath:      "/",
			Path:         "/",
		},
		"ftp://john%20doe@www.google.com/",
	},
	// query
	{
		"http://www.google.com/?q=go+language",
		&URL{
			Raw:          "http://www.google.com/?q=go+language",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/?q=go+language",
			Path:         "/",
			RawQuery:     "q=go+language",
		},
		"",
	},
	// query with hex escaping: NOT parsed
	{
		"http://www.google.com/?q=go%20language",
		&URL{
			Raw:          "http://www.google.com/?q=go%20language",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/?q=go%20language",
			Path:         "/",
			RawQuery:     "q=go%20language",
		},
		"",
	},
	// %20 outside query
	{
		"http://www.google.com/a%20b?q=c+d",
		&URL{
			Raw:          "http://www.google.com/a%20b?q=c+d",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/a%20b?q=c+d",
			Path:         "/a b",
			RawQuery:     "q=c+d",
		},
		"",
	},
	// path without leading /, so no query parsing
	{
		"http:www.google.com/?q=go+language",
		&URL{
			Raw:        "http:www.google.com/?q=go+language",
			Scheme:     "http",
			RawPath:    "www.google.com/?q=go+language",
			Path:       "www.google.com/?q=go+language",
			OpaquePath: true,
		},
		"http:www.google.com/?q=go+language",
	},
	// path without leading /, so no query parsing
	{
		"http:%2f%2fwww.google.com/?q=go+language",
		&URL{
			Raw:        "http:%2f%2fwww.google.com/?q=go+language",
			Scheme:     "http",
			RawPath:    "%2f%2fwww.google.com/?q=go+language",
			Path:       "//www.google.com/?q=go+language",
			OpaquePath: true,
		},
		"http:%2f/www.google.com/?q=go+language",
	},
	// non-authority
	{
		"mailto:/webmaster@golang.org",
		&URL{
			Raw:     "mailto:/webmaster@golang.org",
			Scheme:  "mailto",
			RawPath: "/webmaster@golang.org",
			Path:    "/webmaster@golang.org",
		},
		"",
	},
	// non-authority
	{
		"mailto:webmaster@golang.org",
		&URL{
			Raw:        "mailto:webmaster@golang.org",
			Scheme:     "mailto",
			RawPath:    "webmaster@golang.org",
			Path:       "webmaster@golang.org",
			OpaquePath: true,
		},
		"",
	},
	// unescaped :// in query should not create a scheme
	{
		"/foo?query=http://bad",
		&URL{
			Raw:      "/foo?query=http://bad",
			RawPath:  "/foo?query=http://bad",
			Path:     "/foo",
			RawQuery: "query=http://bad",
		},
		"",
	},
	// leading // without scheme should create an authority
	{
		"//foo",
		&URL{
			RawAuthority: "foo",
			Raw:          "//foo",
			Host:         "foo",
			Scheme:       "",
			RawPath:      "",
			Path:         "",
		},
		"",
	},
	// leading // without scheme, with userinfo, path, and query
	{
		"//user@foo/path?a=b",
		&URL{
			Raw:          "//user@foo/path?a=b",
			RawAuthority: "user@foo",
			RawUserinfo:  "user",
			Scheme:       "",
			RawPath:      "/path?a=b",
			Path:         "/path",
			RawQuery:     "a=b",
			Host:         "foo",
		},
		"",
	},
	// Three leading slashes isn't an authority, but doesn't return an error.
	// (We can't return an error, as this code is also used via
	// ServeHTTP -> ReadRequest -> ParseURL, which is arguably a
	// different URL parsing context, but currently shares the
	// same codepath)
	{
		"///threeslashes",
		&URL{
			RawAuthority: "",
			Raw:          "///threeslashes",
			Host:         "",
			Scheme:       "",
			RawPath:      "///threeslashes",
			Path:         "///threeslashes",
		},
		"",
	},
	{
		"http://user:password@google.com",
		&URL{
			Raw:          "http://user:password@google.com",
			Scheme:       "http",
			RawAuthority: "user:password@google.com",
			RawUserinfo:  "user:password",
			Host:         "google.com",
		},
		"http://user:******@google.com",
	},
	{
		"http://user:longerpass@google.com",
		&URL{
			Raw:          "http://user:longerpass@google.com",
			Scheme:       "http",
			RawAuthority: "user:longerpass@google.com",
			RawUserinfo:  "user:longerpass",
			Host:         "google.com",
		},
		"http://user:******@google.com",
	},
}

var urlnofragtests = []URLTest{
	{
		"http://www.google.com/?q=go+language#foo",
		&URL{
			Raw:          "http://www.google.com/?q=go+language#foo",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/?q=go+language#foo",
			Path:         "/",
			RawQuery:     "q=go+language#foo",
		},
		"",
	},
}

var urlfragtests = []URLTest{
	{
		"http://www.google.com/?q=go+language#foo",
		&URL{
			Raw:          "http://www.google.com/?q=go+language#foo",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/?q=go+language#foo",
			Path:         "/",
			RawQuery:     "q=go+language",
			Fragment:     "foo",
		},
		"",
	},
	{
		"http://www.google.com/?q=go+language#foo%26bar",
		&URL{
			Raw:          "http://www.google.com/?q=go+language#foo%26bar",
			Scheme:       "http",
			RawAuthority: "www.google.com",
			Host:         "www.google.com",
			RawPath:      "/?q=go+language#foo%26bar",
			Path:         "/",
			RawQuery:     "q=go+language",
			Fragment:     "foo&bar",
		},
		"http://www.google.com/?q=go+language#foo&bar",
	},
}

// more useful string for debugging than fmt's struct printer
func ufmt(u *URL) string {
	return fmt.Sprintf("raw=%q, scheme=%q, rawpath=%q, auth=%q, userinfo=%q, host=%q, path=%q, rawq=%q, frag=%q",
		u.Raw, u.Scheme, u.RawPath, u.RawAuthority, u.RawUserinfo,
		u.Host, u.Path, u.RawQuery, u.Fragment)
}

func DoTest(t *testing.T, parse func(string) (*URL, os.Error), name string, tests []URLTest) {
	for _, tt := range tests {
		u, err := parse(tt.in)
		if err != nil {
			t.Errorf("%s(%q) returned error %s", name, tt.in, err)
			continue
		}
		if !reflect.DeepEqual(u, tt.out) {
			t.Errorf("%s(%q):\n\thave %v\n\twant %v\n",
				name, tt.in, ufmt(u), ufmt(tt.out))
		}
	}
}

func TestParseURL(t *testing.T) {
	DoTest(t, ParseURL, "ParseURL", urltests)
	DoTest(t, ParseURL, "ParseURL", urlnofragtests)
}

func TestParseURLReference(t *testing.T) {
	DoTest(t, ParseURLReference, "ParseURLReference", urltests)
	DoTest(t, ParseURLReference, "ParseURLReference", urlfragtests)
}

func DoTestString(t *testing.T, parse func(string) (*URL, os.Error), name string, tests []URLTest) {
	for _, tt := range tests {
		u, err := parse(tt.in)
		if err != nil {
			t.Errorf("%s(%q) returned error %s", name, tt.in, err)
			continue
		}
		s := u.String()
		expected := tt.in
		if len(tt.roundtrip) > 0 {
			expected = tt.roundtrip
		}
		if s != expected {
			t.Errorf("%s(%q).String() == %q (expected %q)", name, tt.in, s, expected)
		}
	}
}

func TestURLString(t *testing.T) {
	DoTestString(t, ParseURL, "ParseURL", urltests)
	DoTestString(t, ParseURL, "ParseURL", urlnofragtests)
	DoTestString(t, ParseURLReference, "ParseURLReference", urltests)
	DoTestString(t, ParseURLReference, "ParseURLReference", urlfragtests)
}

type URLEscapeTest struct {
	in  string
	out string
	err os.Error
}

var unescapeTests = []URLEscapeTest{
	{
		"",
		"",
		nil,
	},
	{
		"abc",
		"abc",
		nil,
	},
	{
		"1%41",
		"1A",
		nil,
	},
	{
		"1%41%42%43",
		"1ABC",
		nil,
	},
	{
		"%4a",
		"J",
		nil,
	},
	{
		"%6F",
		"o",
		nil,
	},
	{
		"%", // not enough characters after %
		"",
		URLEscapeError("%"),
	},
	{
		"%a", // not enough characters after %
		"",
		URLEscapeError("%a"),
	},
	{
		"%1", // not enough characters after %
		"",
		URLEscapeError("%1"),
	},
	{
		"123%45%6", // not enough characters after %
		"",
		URLEscapeError("%6"),
	},
	{
		"%zzzzz", // invalid hex digits
		"",
		URLEscapeError("%zz"),
	},
}

func TestURLUnescape(t *testing.T) {
	for _, tt := range unescapeTests {
		actual, err := URLUnescape(tt.in)
		if actual != tt.out || (err != nil) != (tt.err != nil) {
			t.Errorf("URLUnescape(%q) = %q, %s; want %q, %s", tt.in, actual, err, tt.out, tt.err)
		}
	}
}

var escapeTests = []URLEscapeTest{
	{
		"",
		"",
		nil,
	},
	{
		"abc",
		"abc",
		nil,
	},
	{
		"one two",
		"one+two",
		nil,
	},
	{
		"10%",
		"10%25",
		nil,
	},
	{
		" ?&=#+%!<>#\"{}|\\^[]`☺\t",
		"+%3f%26%3d%23%2b%25!%3c%3e%23%22%7b%7d%7c%5c%5e%5b%5d%60%e2%98%ba%09",
		nil,
	},
}

func TestURLEscape(t *testing.T) {
	for _, tt := range escapeTests {
		actual := URLEscape(tt.in)
		if tt.out != actual {
			t.Errorf("URLEscape(%q) = %q, want %q", tt.in, actual, tt.out)
		}

		// for bonus points, verify that escape:unescape is an identity.
		roundtrip, err := URLUnescape(actual)
		if roundtrip != tt.in || err != nil {
			t.Errorf("URLUnescape(%q) = %q, %s; want %q, %s", actual, roundtrip, err, tt.in, "[no error]")
		}
	}
}

type CanonicalPathTest struct {
	in  string
	out string
}

var canonicalTests = []CanonicalPathTest{
	{"", ""},
	{"/", "/"},
	{".", ""},
	{"./", ""},
	{"/a/", "/a/"},
	{"a/", "a/"},
	{"a/./", "a/"},
	{"./a", "a"},
	{"/a/../b", "/b"},
	{"a/../b", "b"},
	{"a/../../b", "../b"},
	{"a/.", "a/"},
	{"../.././a", "../../a"},
	{"/../.././a", "/../../a"},
	{"a/b/g/../..", "a/"},
	{"a/b/..", "a/"},
	{"a/b/.", "a/b/"},
	{"a/b/../../../..", "../.."},
	{"a./", "a./"},
	{"/../a/b/../../../", "/../../"},
	{"../a/b/../../../", "../../"},
}

func TestCanonicalPath(t *testing.T) {
	for _, tt := range canonicalTests {
		actual := CanonicalPath(tt.in)
		if tt.out != actual {
			t.Errorf("CanonicalPath(%q) = %q, want %q", tt.in, actual, tt.out)
		}
	}
}

type UserinfoTest struct {
	User     string
	Password string
	Raw      string
}

var userinfoTests = []UserinfoTest{
	{"user", "password", "user:password"},
	{"foo:bar", "~!@#$%^&*()_+{}|[]\\-=`:;'\"<>?,./",
		"foo%3abar:~!%40%23$%25%5e&*()_+%7b%7d%7c%5b%5d%5c-=%60%3a;'%22%3c%3e?,.%2f"},
}

func TestEscapeUserinfo(t *testing.T) {
	for _, tt := range userinfoTests {
		if raw := EscapeUserinfo(tt.User, tt.Password); raw != tt.Raw {
			t.Errorf("EscapeUserinfo(%q, %q) = %q, want %q", tt.User, tt.Password, raw, tt.Raw)
		}
	}
}

func TestUnescapeUserinfo(t *testing.T) {
	for _, tt := range userinfoTests {
		if user, pass, err := UnescapeUserinfo(tt.Raw); user != tt.User || pass != tt.Password || err != nil {
			t.Errorf("UnescapeUserinfo(%q) = %q, %q, %v, want %q, %q, nil", tt.Raw, user, pass, err, tt.User, tt.Password)
		}
	}
}

func TestCleanURLForHTTPRequest(t *testing.T) {
	path := "//user@foo/bar/"
	url, _ := ParseURL(path)
	if url.RawAuthority != "user@foo" {
		t.Errorf("Expected authority of 'foo'; got %q", url.RawAuthority)
	}
	if url.RawUserinfo != "user" {
		t.Errorf("Expected userinfo of 'user'; got %q", url.RawUserinfo)
	}
	cleanURLForHTTPRequest(url)
	if url.RawAuthority != "" {
		t.Errorf("Expected blank authority.")
	}
	if url.RawUserinfo != "" {
		t.Errorf("Expected blank userinfo.")
	}
	if url.Host != "" {
		t.Errorf("Expected blank host.")
	}
	if url.RawPath != path {
		t.Errorf("Expected path %q; got %q", path, url.RawPath)
	}
	if url.Path != path {
		t.Errorf("Expected path %q; got %q", path, url.Path)
	}
}

func mustParseURL(t *testing.T, url string) *URL {
	u, err := ParseURLReference(url)
	if err != nil {
		t.Fatalf("Expected URL to parse: %q, got error: %v", url, err)
	}
	return u
}

func TestApplyReferenceSegments(t *testing.T) {
	tests := []struct {
		base, ref, expected string
	}{
		{"a/b", ".", "a/"},
		{"a/b", "c", "a/c"},
		{"a/b", "..", ""},
		{"a/", "..", ""},
		{"a/", "../..", ""},
		{"a/b/c", "..", "a/"},
		{"a/b/c", "../d", "a/d"},
		{"a/b/c", ".././d", "a/d"},
		{"a/b", "./..", ""},
	}
	for _, test := range tests {
		segs := strings.Split(test.base, "/", -1)
		refSegs := strings.Split(test.ref, "/", -1)
		got := strings.Join(applyReferenceSegments(segs, refSegs), "/")
		if got != test.expected {
			t.Errorf("For %q + %q got %q; expected %q", test.base, test.ref, got, test.expected)
		}
	}
}

func TestURLAdd(t *testing.T) {
	tests := []struct {
		base, rel, expected string
	}{
		// Absolute URL references
		{"http://foo.com?a=b", "https://bar.com/", "https://bar.com/"},
		{"http://foo.com/", "https://bar.com/?a=b", "https://bar.com/?a=b"},
		{"http://foo.com/bar", "mailto:foo@example.com", "mailto:foo@example.com"},

		// Path-absolute references
		{"http://foo.com/bar", "/baz", "http://foo.com/baz"},
		{"http://foo.com/bar?a=b#f", "/baz", "http://foo.com/baz"},
		{"http://foo.com/bar?a=b", "/baz?c=d", "http://foo.com/baz?c=d"},

		// Scheme-relative
		{"https://foo.com/bar?a=b", "//bar.com/quux", "https://bar.com/quux"},

		// Path-relative references:

		// ... current directory
		{"http://foo.com", ".", "http://foo.com/"},
		{"http://foo.com/bar", ".", "http://foo.com/"},
		{"http://foo.com/bar/", ".", "http://foo.com/bar/"},

		// ... going down
		{"http://foo.com", "bar", "http://foo.com/bar"},
		{"http://foo.com/", "bar", "http://foo.com/bar"},
		{"http://foo.com/bar/baz", "quux", "http://foo.com/bar/quux"},

		// ... going up
		{"http://foo.com/bar/baz", "../quux", "http://foo.com/quux"},
		{"http://foo.com/bar/baz", "../../../../../quux", "http://foo.com/quux"},
		{"http://foo.com/bar", "..", "http://foo.com/"},
		{"http://foo.com/bar/baz", "./..", "http://foo.com/"},

		// Triple dot isn't special
		{"http://foo.com/bar", "...", "http://foo.com/..."},

		// Fragment
		{"http://foo.com/bar", ".#frag", "http://foo.com/#frag"},
	}
	for _, test := range tests {
		base := mustParseURL(t, test.base)
		rel := mustParseURL(t, test.rel)
		url := base.Add(rel)
		urlStr := url.String()
		if urlStr != test.expected {
			t.Errorf("Adding %q + %q != %q; got %q", test.base, test.rel, test.expected, urlStr)
		}
	}
}
