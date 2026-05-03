package bin

import (
	"context"
	"runtime"
	"sort"
	"strings"
)

type ResolverFunc func(context.Context, ToolSpec) (*ResolvedArtifact, error)

type ToolSpec struct {
	Name              string
	DisplayName       string
	Description       string
	BinaryName        string
	BundledBinaries   []string
	BundledFiles      []string
	VersionArgs       []string
	AvailabilityCheck func() error
	FinalizeInstall   FinalizeInstallFunc
	ResolveLatest     ResolverFunc
}

func (t ToolSpec) InstalledFilename() string {
	return binaryInstallFilename(t.BinaryName)
}

func binaryInstallFilename(binaryName string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(binaryName), ".exe") {
		return binaryName + ".exe"
	}
	return binaryName
}

type ArchiveType string

const (
	ArchiveTypeBinary ArchiveType = "binary"
	ArchiveTypeZip    ArchiveType = "zip"
	ArchiveTypeTarGz  ArchiveType = "tar.gz"
	ArchiveTypeTarXz  ArchiveType = "tar.xz"
)

type ResolvedArtifact struct {
	SourceType     string
	Version        string
	ReleaseTag     string
	ManifestURL    string
	AssetName      string
	DownloadURL    string
	ArchiveType    ArchiveType
	BinaryPath     string
	ChecksumSHA256 string
	Resolution     *ResolutionInfo
}

var catalog = map[string]ToolSpec{
	"ast-grep": {
		Name:          "ast-grep",
		DisplayName:   "ast-grep",
		Description:   "A fast and polyglast tool for code searching, linting, rewriting at large scale",
		BinaryName:    "sg",
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveAstGrepLatest,
	},
	"universal-ctags": {
		Name:          "universal-ctags",
		DisplayName:   "Universal Ctags",
		Description:   "A maintained ctags implementation for source code indexing",
		BinaryName:    "ctags",
		ResolveLatest: resolveCtagsLatest,
	},
	"difftastic": {
		Name:          "difftastic",
		DisplayName:   "difftastic",
		Description:   "A structural diff that understands syntax",
		BinaryName:    "difft",
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveDifftasticLatest,
	},
	"fd": {
		Name:          "fd",
		DisplayName:   "fd",
		Description:   "A simple, fast and user-friendly alternative to find",
		BinaryName:    "fd",
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveFDLatest,
	},
	"ripgrep": {
		Name:          "ripgrep",
		DisplayName:   "ripgrep",
		Description:   "Recursively searches directories for a regex pattern",
		BinaryName:    "rg",
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveRipgrepLatest,
	},
	"nushell": {
		Name:        "nushell",
		DisplayName: "Nushell",
		Description: "A new type of shell",
		BinaryName:  "nu",
		BundledBinaries: []string{
			"nu_plugin_formats",
			"nu_plugin_gstat",
			"nu_plugin_query",
		},
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveNushellLatest,
	},
	"jbang": {
		Name:              "jbang",
		DisplayName:       "JBang",
		Description:       "Java scripting and app launcher (requires a system JDK)",
		BinaryName:        "jbang",
		BundledFiles:      []string{"jbang.jar"},
		VersionArgs:       []string{"--version"},
		AvailabilityCheck: jbangAvailabilityCheck,
		FinalizeInstall:   finalizeJBangInstall,
		ResolveLatest:     resolveJBangLatest,
	},
	"nu-plugin-semver": {
		Name:          "nu-plugin-semver",
		DisplayName:   "nu_plugin_semver",
		Description:   "Nushell plugin for semantic version parsing and manipulation",
		BinaryName:    "nu_plugin_semver",
		ResolveLatest: resolveThirdPartyNuPlugin("abusch", "nu_plugin_semver"),
	},
	"nu-plugin-file": {
		Name:          "nu-plugin-file",
		DisplayName:   "nu_plugin_file",
		Description:   "Nushell plugin for determining file types using libmagic",
		BinaryName:    "nu_plugin_file",
		ResolveLatest: resolveThirdPartyNuPlugin("fdncred", "nu_plugin_file"),
	},
	"scc": {
		Name:          "scc",
		DisplayName:   "scc",
		Description:   "Sloc Cloc and Code: a fast accurate code counter with complexity calculations",
		BinaryName:    "scc",
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveSCCLatest,
	},
	"shellcheck": {
		Name:          "shellcheck",
		DisplayName:   "shellcheck",
		Description:   "A static analysis tool for shell scripts",
		BinaryName:    "shellcheck",
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveShellcheckLatest,
	},
	"yq": {
		Name:          "yq",
		DisplayName:   "yq",
		Description:   "A lightweight and portable command-line YAML, JSON, XML, CSV, TSV and properties processor",
		BinaryName:    "yq",
		VersionArgs:   []string{"--version"},
		ResolveLatest: resolveYqLatest,
	},
}

func GetTool(name string) (ToolSpec, bool) {
	spec, ok := catalog[strings.ToLower(strings.TrimSpace(name))]
	return spec, ok
}

func AllTools() []ToolSpec {
	tools := make([]ToolSpec, 0, len(catalog))
	for _, spec := range catalog {
		tools = append(tools, spec)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}
