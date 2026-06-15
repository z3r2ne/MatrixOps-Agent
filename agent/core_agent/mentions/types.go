package mentions

type FileMention struct {
	RawURL string
	Path   string
}

type WorkerMention struct {
	RawURL string
	Name   string
}

type SkillMention struct {
	RawURL string
	Name   string
}

type CommandMention struct {
	RawURL string
	Name   string
}

type ReviewParams struct {
	FromType string
	From     string
	ToType   string
	To       string
}

type ReviewMention struct {
	RawURL string
	Text   string
	Params ReviewParams
}
