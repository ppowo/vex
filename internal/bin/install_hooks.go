package bin

type FinalizeInstallFunc func(InstallLayout) error

type InstallLayout struct {
	BinDir     string
	TargetPath string
	Artifact   *ResolvedArtifact
}
