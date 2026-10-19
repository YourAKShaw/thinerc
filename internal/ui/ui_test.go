package ui

import (
	"strings"
	"testing"
	"time"
)

func TestS8_SanitizeStripsEscapesAndControls(t *testing.T) {
	cases := map[string]string{
		"node":                      "node",
		"\x1b]0;pwned\x07evil":      "evil",
		"\x1b[31mred\x1b[0m":        "red",
		"a\x00b\x07c\rd":            "abcd",
		"tab\there":                 "tab here",
		"line\u2028break\u0085":     "linebreak",
		"\x1b[2J\x1b[Hclear-screen": "clear-screen",
		"\x1bP+q544e\x1b\\after":    "+q544eafter",
	}
	for in, want := range cases {
		if got := Sanitize(in, 0); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	// Whatever the input, no ESC may survive.
	for in := range cases {
		if strings.ContainsRune(Sanitize(in, 0), 0x1b) {
			t.Errorf("ESC survived for %q", in)
		}
	}
	if got := Sanitize(strings.Repeat("x", 50), 10); got != strings.Repeat("x", 10)+"…" {
		t.Errorf("length bound not applied: %q", got)
	}
}

func TestRenderRespectsColorLevel(t *testing.T) {
	s := S().Bold().Fg(Hex("#ff0000"))
	if got := s.RenderAt(ColorNone, "x"); got != "x" {
		t.Errorf("ColorNone should not emit escapes, got %q", got)
	}
	if got := s.RenderAt(ColorTrue, "x"); got != "\x1b[1;38;2;255;0;0mx\x1b[0m" {
		t.Errorf("truecolor render = %q", got)
	}
	if got := s.RenderAt(Color16, "x"); !strings.Contains(got, ";91m") && !strings.Contains(got, ";31m") {
		t.Errorf("16-colour render should map to red, got %q", got)
	}
}

func TestTo256(t *testing.T) {
	if got := to256(RGB{0, 0, 0}); got != 16 {
		t.Errorf("black → %d, want 16", got)
	}
	if got := to256(RGB{255, 255, 255}); got != 231 && got != 255 {
		t.Errorf("white → %d", got)
	}
	if got := to256(RGB{128, 128, 128}); got < 232 {
		t.Errorf("mid grey should use the grey ramp, got %d", got)
	}
}

func TestGradientEndpoints(t *testing.T) {
	a, b := Hex("#000000"), Hex("#ffffff")
	if Along([]RGB{a, b}, 0) != a || Along([]RGB{a, b}, 1) != b {
		t.Fatal("gradient endpoints wrong")
	}
	if got := GradientAt(ColorNone, "hi", 0, a, b); got != "hi" {
		t.Errorf("no-colour gradient = %q", got)
	}
	if Width(GradientAt(ColorTrue, "hello", 0.3, a, b)) != 5 {
		t.Error("gradient changed the display width")
	}
}

func TestWordmarkAligned(t *testing.T) {
	rows := Wordmark()
	w := Width(rows[0])
	for i, r := range rows {
		if Width(r) != w {
			t.Errorf("row %d width %d, want %d", i, Width(r), w)
		}
	}
}

func TestParseSelection(t *testing.T) {
	got, err := ParseSelection("1, 3-4", 5)
	if err != nil || len(got) != 3 || got[0] != 0 || got[2] != 3 {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, _ := ParseSelection("all", 3); len(got) != 3 {
		t.Errorf("all → %v", got)
	}
	if got, _ := ParseSelection("", 3); got != nil {
		t.Errorf("empty must select nothing, got %v", got)
	}
	if _, err := ParseSelection("9", 3); err == nil {
		t.Error("out-of-range selection accepted")
	}
}

func TestHumanBytesAndDuration(t *testing.T) {
	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KiB", 3 << 30: "3.0 GiB", 150 << 20: "150 MiB"}
	for n, want := range cases {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", n, got, want)
		}
	}
	if got := HumanDuration(2*time.Hour + 14*time.Minute); got != "2h14m" {
		t.Errorf("duration = %q", got)
	}
}

func TestMiddleTruncate(t *testing.T) {
	got := MiddleTruncate("/home/ayush/code/very/long/path/shop", 16)
	if Width(got) > 16 || !strings.Contains(got, "…") || !strings.HasSuffix(got, "shop") {
		t.Errorf("MiddleTruncate = %q", got)
	}
}

func TestThemesComplete(t *testing.T) {
	for _, n := range ThemeNames() {
		p := ThemeByName(n)
		if len(p.Brand) < 2 || p.Name != n || p.Title == "" {
			t.Errorf("theme %q incomplete", n)
		}
	}
	if ThemeByName("nope").Name != "thine" {
		t.Error("unknown theme must fall back to thine")
	}
}
