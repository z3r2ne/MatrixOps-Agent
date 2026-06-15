package command

import (
	_ "embed"
	"regexp"
	"strings"

	"matrixops-agent/config"
	"matrixops-agent/taskctx"
	"pkgs/db/models"
)

type Info struct {
	Name        string
	Description string
	Agent       string
	Model       string
	MCP         bool
	Template    string
	Subtask     bool
	Hints       []string
}

//go:embed template/initialize.txt
var promptInitialize string

//go:embed template/review.txt
var promptReview string

const (
	DefaultInit   = "init"
	DefaultReview = "review"
)

func loadState(task *models.Task) (map[string]Info, error) {
	ctx, err := taskctx.Resolve(task)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Get(task)
	if err != nil {
		return nil, err
	}
	result := map[string]Info{
		DefaultInit: {
			Name:        DefaultInit,
			Description: "create/update AGENTS.md",
			Template:    strings.ReplaceAll(promptInitialize, "${path}", ctx.Worktree),
		},
		DefaultReview: {
			Name:        DefaultReview,
			Description: "review changes [commit|branch|pr], defaults to uncommitted",
			Template:    strings.ReplaceAll(promptReview, "${path}", ctx.Worktree),
			Subtask:     true,
		},
	}
	for name, command := range cfg.Command {
		result[name] = Info{
			Name:        name,
			Description: command.Description,
			Agent:       command.Agent,
			Model:       command.Model,
			Template:    command.Template,
			Subtask:     command.Subtask,
			MCP:         command.MCP,
		}
	}
	for key, value := range result {
		value.Hints = Hints(value.Template)
		result[key] = value
	}
	return result, nil
}

func Get(task *models.Task, name string) (*Info, error) {
	all, err := loadState(task)
	if err != nil {
		return nil, err
	}
	if item, ok := all[name]; ok {
		return &item, nil
	}
	return nil, nil
}

func List(task *models.Task) ([]Info, error) {
	all, err := loadState(task)
	if err != nil {
		return nil, err
	}
	items := []Info{}
	for _, value := range all {
		items = append(items, value)
	}
	return items, nil
}

func Hints(template string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, match := range placeholderMatches(template) {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		result = append(result, match)
	}
	if strings.Contains(template, "$ARGUMENTS") {
		if _, ok := seen["$ARGUMENTS"]; !ok {
			result = append(result, "$ARGUMENTS")
		}
	}
	return result
}

func placeholderMatches(template string) []string {
	matches := []string{}
	for _, item := range placeholderRegex.FindAllString(template, -1) {
		matches = append(matches, item)
	}
	return matches
}

var placeholderRegex = regexp.MustCompile(`\$\d+`)
