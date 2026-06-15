package util

// MergeMaps performs a deep merge of src into dst, overwriting non-map values.
func MergeMaps(dst map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = map[string]interface{}{}
	}
	for key, value := range src {
		srcMap, ok := value.(map[string]interface{})
		if !ok {
			dst[key] = value
			continue
		}
		dstMap, ok := dst[key].(map[string]interface{})
		if !ok {
			dst[key] = MergeMaps(nil, srcMap)
			continue
		}
		dst[key] = MergeMaps(dstMap, srcMap)
	}
	return dst
}
