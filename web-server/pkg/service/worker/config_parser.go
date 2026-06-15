package worker

// ToString 将 interface{} 转换为字符串
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
