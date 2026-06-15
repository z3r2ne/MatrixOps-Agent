package models

const (
	TaskMemoryLibraryModeNone       = "none"
	TaskMemoryLibraryModeTemporary  = "temporary"
	TaskMemoryLibraryModeLibraries  = "libraries"
)

func NormalizeTaskMemoryLibraryMode(value string) string {
	switch value {
	case TaskMemoryLibraryModeTemporary, TaskMemoryLibraryModeLibraries:
		return value
	default:
		return TaskMemoryLibraryModeNone
	}
}
