package imagedbutil

import "strings"

// returns the file extension in lower-case.
// todo: special case for .tar.xz etc maybe.
func GetExt(path string) string {
	splitName := strings.Split(path, ".")
	ext := strings.ToLower(splitName[len(splitName)-1])
	return ext
}
