package tool

import (
	"encoding/json"
	"fmt"
)

type QuestionTool struct {
	waitUserInput func(map[string]interface{}) (map[string]any, error)
}

func NewQuestionTool(waitUserInput func(map[string]interface{}) (map[string]any, error)) *QuestionTool {
	return &QuestionTool{
		waitUserInput: waitUserInput,
	}
}

var _ Tool = (*QuestionTool)(nil)

func (QuestionTool) Name() string {
	return "question"
}

func (QuestionTool) VerbosName() string {
	return "用户提问"
}

func (QuestionTool) Description() string {
	return "向用户发起问题"
}

func (QuestionTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"questions": map[string]interface{}{
			"type":        "array",
			"description": "需要向用户询问的问题列表",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "向用户提问的问题",
					},
					"options": map[string]interface{}{
						"type":        "array",
						"description": "给用户的示例选项，最多3个（用户可以自定义输入内容）",
						"items": map[string]interface{}{
							"type": "string",
						},
						"maxItems": 3,
					},
					"required": map[string]interface{}{
						"type":        "boolean",
						"description": "是否必须回答此问题",
					},
				},
				"required": []string{"question"},
			},
		},
	}, []string{"questions"})
}

func (q *QuestionTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	// questionsRaw, ok := input["questions"]
	// if !ok {
	// 	return Result{IsError: true}, errors.New("question: missing questions")
	// }
	// questions, ok := questionsRaw.(map[string]interface{})
	// if !ok {
	// 	return Result{IsError: true}, errors.New("question: questions is not a map")
	// }
	answers, err := q.waitUserInput(input)
	if err != nil {
		return Result{IsError: true}, err
	}

	encoded, err := json.Marshal(answers)
	if err != nil {
		return Result{IsError: true}, err
	}
	return Result{Content: fmt.Sprintf("用户回答: %s", string(encoded))}, nil
}
