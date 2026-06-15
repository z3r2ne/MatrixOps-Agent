package git

type BranchInfo struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
	IsRemote  bool   `json:"isRemote"`
}

type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Head   string `json:"head"`
}

type RepoState struct {
	CommitHash     string   `json:"commitHash"`
	Branch         string   `json:"branch"`
	IsDirty        bool     `json:"isDirty"`
	ModifiedFiles  []string `json:"modifiedFiles"`
	UntrackedFiles []string `json:"untrackedFiles"`
	ModifiedCount  int      `json:"modifiedCount"`
	UntrackedCount int      `json:"untrackedCount"`
}

type Result struct {
	Type  string                   `json:"type"`
	Diff  string                   `json:"diff"`
	Files []map[string]interface{} `json:"files"`
}
