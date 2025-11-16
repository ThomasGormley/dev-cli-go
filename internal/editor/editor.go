package editor

import (
	"os"
	"strings"
)

func Lookup() (editor string, args []string, found bool) {
	editor, found = os.LookupEnv("EDITOR")
	if editor == "" {
		return "", []string{}, false
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return "", []string{}, false
	}
	return parts[0], parts[1:], found
}
