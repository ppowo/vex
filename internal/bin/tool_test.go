package bin

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal", a: "1.0.8", b: "1.0.8", want: 0},
		{name: "newer patch", a: "1.0.9", b: "1.0.8", want: 1},
		{name: "older minor", a: "1.1.0", b: "1.2.0", want: -1},
		{name: "trim leading v", a: "v1.0.8", b: "1.0.8", want: 0},
		{name: "stable beats prerelease", a: "1.0.8", b: "1.0.8-rc1", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			switch {
			case tt.want < 0 && got >= 0:
				t.Fatalf("CompareVersions(%q, %q) = %d, want negative", tt.a, tt.b, got)
			case tt.want > 0 && got <= 0:
				t.Fatalf("CompareVersions(%q, %q) = %d, want positive", tt.a, tt.b, got)
			case tt.want == 0 && got != 0:
				t.Fatalf("CompareVersions(%q, %q) = %d, want 0", tt.a, tt.b, got)
			}
		})
	}
}

func TestDetectArchiveType(t *testing.T) {
	tests := map[string]ArchiveType{
		"tool.tar.xz": ArchiveTypeTarXz,
		"tool.tar.gz": ArchiveTypeTarGz,
		"tool.zip":    ArchiveTypeZip,
		"tool":        ArchiveTypeBinary,
	}

	for name, want := range tests {
		if got := DetectArchiveType(name); got != want {
			t.Fatalf("DetectArchiveType(%q) = %q, want %q", name, got, want)
		}
	}
}
