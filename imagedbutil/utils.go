package imagedbutil

import "strings"

// returns the file extension in lower-case.
// todo: special case for .tar.xz etc maybe.
func GetExt(path string) string {
	splitName := strings.Split(path, ".")
	ext := strings.ToLower(splitName[len(splitName)-1])
	return ext
}

func MidTruncateString(str string, maxlen int) string {
	var trimPosition = int(maxlen / 2) // X characters before the end of string
	const trimStr = "[…]"
	strlen := len(str)

	if len(str) <= maxlen {
		return str
	}
	if maxlen < trimPosition {
		return str[:maxlen-len(trimStr)] + trimStr
	}
	prefix := str[:(maxlen - (trimPosition + len(trimStr)))]
	return prefix + trimStr + str[strlen-trimPosition:]
}
