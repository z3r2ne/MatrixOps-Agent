package coreagent

type NoEmitter struct{}

func (NoEmitter) UpdateMessage(info *Message) (*Message, error) { return info, nil }
func (NoEmitter) UpdatePart(part *Part) (*Part, error)          { return part, nil }
func (NoEmitter) Emit(name string, payload interface{})         {}
