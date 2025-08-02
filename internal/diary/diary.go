package diary

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"
)

func DateStringsFor(t time.Time) (year, month, full string) {
	year = t.Format("2006")
	month = t.Format("01")
	full = t.Format("2006-01-02")
	return
}

func RepoPath() (string, bool) {
	// return os.LookupEnv("DIARY_REPO")
	var diaryDir = path.Join(os.Getenv("HOME"), "dev", "engineering-diary")
	return diaryDir, true
}

func EntryPathFor(t time.Time) (string, error) {
	year, month, full := DateStringsFor(t)

	repo, ok := RepoPath()
	if !ok {
		return "", fmt.Errorf("DIARY_REPO environment variable not set")
	}

	return path.Join(repo, "docs", year, month, fmt.Sprintf("%s.md", full)), nil
}

func EntryExists(t time.Time) bool {
	path, err := EntryPathFor(t)

	if err != nil {
		return false
	}

	_, err = os.Stat(path)
	return err == nil
}

func EnsureEntryExists(t time.Time) (string, error) {
	entryPath, err := EntryPathFor(t)
	if err != nil {
		return "", err
	}

	// Check if the entry file exists
	_, statErr := os.Stat(entryPath)
	if statErr == nil {
		return entryPath, nil
	}

	// If not exists, create parent directories and the file
	dir := path.Dir(entryPath)
	if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
		return "", mkErr
	}

	if err := Create(); err != nil {
		return "", err
	}

	return entryPath, nil
}

func Create() error {
	repo, ok := RepoPath()
	if !ok {
		return errors.New("unable to find diary repo")
	}
	return exec.Command(path.Join(repo, "scripts", "new-entry.sh")).Run()

}
