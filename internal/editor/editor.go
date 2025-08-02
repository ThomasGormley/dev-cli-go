package editor

import (
	"os"
	"strings"
)

func Lookup() (editor string, args []string, found bool) {
	editor, found = os.LookupEnv("EDITOR")
	if editor == "" {
		return "", nil, false
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return "", nil, false
	}
	return parts[0], parts[1:], found
}
