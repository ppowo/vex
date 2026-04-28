package bin

import "testing"

func TestCtagsNightlyVersion(t *testing.T) {
	tests := []struct {
		name        string
		tagName     string
		releaseName string
		want        string
	}{
		{
			name:    "nightly tag with commit suffix",
			tagName: "2026.04.20+0498b5983b38f835ece70890ea171d1c1204f284",
			want:    "2026.04.20",
		},
		{
			name:        "falls back to release name",
			releaseName: "2026.04.20+0498b5983b38f835ece70890ea171d1c1204f284",
			want:        "2026.04.20",
		},
		{
			name:    "strips leading v",
			tagName: "v6.2.1",
			want:    "6.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ctagsNightlyVersion(tt.tagName, tt.releaseName); got != tt.want {
				t.Fatalf("ctagsNightlyVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCtagsAssetNameForGOOSArch(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		goarch  string
		want    string
		wantErr bool
	}{
		{
			name:   "linux amd64",
			goos:   "linux",
			goarch: "amd64",
			want:   "uctags-2026.04.20-linux-x86_64.release.tar.xz",
		},
		{
			name:   "linux arm64",
			goos:   "linux",
			goarch: "arm64",
			want:   "uctags-2026.04.20-linux-aarch64.release.tar.xz",
		},
		{
			name:   "darwin amd64",
			goos:   "darwin",
			goarch: "amd64",
			want:   "uctags-2026.04.20-macos-10.15-x86_64.release.tar.xz",
		},
		{
			name:   "darwin arm64",
			goos:   "darwin",
			goarch: "arm64",
			want:   "uctags-2026.04.20-macos-10.15-arm64.release.tar.xz",
		},
		{
			name:    "unsupported os",
			goos:    "windows",
			goarch:  "amd64",
			wantErr: true,
		},
		{
			name:    "unsupported arch",
			goos:    "linux",
			goarch:  "386",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctagsAssetNameForGOOSArch("2026.04.20", tt.goos, tt.goarch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("asset name = %q, want %q", got, tt.want)
			}
		})
	}
}
