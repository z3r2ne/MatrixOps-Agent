package session

func SummaryFlag(info *MessageInfo) bool {
	if info.Summary == nil {
		return false
	}
	if flag, ok := info.Summary.(bool); ok {
		return flag
	}
	return false
}
