package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/urfave/cli/v2"
)

type TestInfo struct {
	Name        string
	PackagePath string
	FileName    string
	IsPackage   bool // true if this represents a whole package
}

var failedTestsFile = os.Getenv("HOME") + "/.dev_cli_test_failed"

func handleTest(stdout, stderr io.Writer) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		goTest := goTest{
			flags: testFlags{
				verbose: ctx.Bool("verbose"),
				rerun:   ctx.Bool("rerun"),
			},
			stdin:  os.Stdin,
			stdout: stdout,
			stderr: stderr,
			env:    os.Environ(),
		}

		if goTest.flags.rerun {
			path, args, err := goTest.readCommandHistory(0)
			if err == nil {
				return goTest.run(ctx.Context, path, args...)
			}

			fmt.Fprintf(stdout, "couldn't determine the previous command")
		}

		if ctx.Bool("all") {
			return goTest.run(ctx.Context, "./...")
		}

		if ctx.Bool("failed") {
			// TODO: Implement runFailedTests function
			fmt.Fprintf(stdout, "failed tests functionality not yet implemented\n")
			return nil
		}

		selectedTest, err := promptForTest()
		if err != nil {
			return err
		}

		return runSelectedTest(ctx, goTest, selectedTest)
	}
}

func promptForTest() (TestInfo, error) {
	tests, err := listTestsFromProject()
	if err != nil {
		return TestInfo{}, err
	}

	testOptions, testLookup := buildTestOptions(tests)

	var testName string
	prompt := &survey.Select{
		Message:  "Choose a test:",
		Options:  testOptions,
		Filter:   contains,
		PageSize: 16,
	}

	if err := survey.AskOne(prompt, &testName); err != nil {
		return TestInfo{}, err
	}

	selectedTest, exists := testLookup[testName]
	if !exists {
		return TestInfo{}, fmt.Errorf("selected test %s not found in lookup", testName)
	}

	return selectedTest, nil
}

func buildTestOptions(tests []TestInfo) ([]string, map[string]TestInfo) {
	packageTests := groupTestsByPackage(tests)
	packages := sortedPackageNames(packageTests)

	var testOptions []string
	testLookup := make(map[string]TestInfo)

	for _, pkg := range packages {
		testsInPackage := packageTests[pkg]

		// Add package-level option if there are multiple tests
		if len(testsInPackage) > 1 {
			packageOption := fmt.Sprintf("📦 %s (all %d tests)", pkg, len(testsInPackage))
			testOptions = append(testOptions, packageOption)
			testLookup[packageOption] = TestInfo{
				Name:        "",
				PackagePath: "./" + pkg + "/...",
				FileName:    "",
				IsPackage:   true,
			}
		}

		// Add individual test options for this package
		for _, test := range testsInPackage {
			uniqueName := fmt.Sprintf(" 🧪 %s", test.Name)
			testOptions = append(testOptions, uniqueName)
			testLookup[uniqueName] = test
		}
	}

	return testOptions, testLookup
}

func groupTestsByPackage(tests []TestInfo) map[string][]TestInfo {
	packageTests := make(map[string][]TestInfo)
	for _, test := range tests {
		pkg := strings.TrimPrefix(strings.TrimSuffix(test.PackagePath, "/..."), "./")
		packageTests[pkg] = append(packageTests[pkg], test)
	}
	return packageTests
}

func sortedPackageNames(packageTests map[string][]TestInfo) []string {
	var packages []string
	for pkg := range packageTests {
		packages = append(packages, pkg)
	}

	// Simple bubble sort for consistency
	for i := 0; i < len(packages); i++ {
		for j := i + 1; j < len(packages); j++ {
			if packages[i] > packages[j] {
				packages[i], packages[j] = packages[j], packages[i]
			}
		}
	}

	return packages
}

func runSelectedTest(ctx *cli.Context, goTest goTest, selectedTest TestInfo) error {
	if selectedTest.IsPackage {
		return goTest.run(ctx.Context, selectedTest.PackagePath)
	}

	runPattern := buildRunPattern(selectedTest.Name)
	return goTest.run(ctx.Context, selectedTest.PackagePath, "-run", runPattern)
}

func contains(filterValue string, optValue string, optIndex int) bool {
	// only include the option if it includes the filter (case insensitive)
	return strings.Contains(strings.ToLower(optValue), strings.ToLower(filterValue))
}

func listTests(reader io.Reader) ([]TestInfo, error) {
	var tests []TestInfo

	// Parse the ripgrep output line by line
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Format: filename:line_number:func TestName(
		parts := bytes.SplitN([]byte(line), []byte(":"), 3)
		if len(parts) < 3 {
			continue
		}

		filename := string(parts[0])
		content := string(parts[2])

		// Extract test function name
		// Looking for "func TestXxx(" pattern
		if bytes.Contains([]byte(content), []byte("func Test")) {
			start := bytes.Index([]byte(content), []byte("func "))
			if start == -1 {
				continue
			}

			// Find the function name
			funcStart := start + 5 // len("func ")
			funcEnd := bytes.Index([]byte(content[funcStart:]), []byte("("))
			if funcEnd == -1 {
				continue
			}

			testName := string(content[funcStart : funcStart+funcEnd])
			// Only add if it starts with "Test" and is not "TestMain"
			if bytes.HasPrefix([]byte(testName), []byte("Test")) && testName != "TestMain" {
				// Extract package path from filename
				packagePath := extractPackagePath(filename)

				tests = append(tests, TestInfo{
					Name:        testName,
					PackagePath: packagePath,
					FileName:    filename,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning ripgrep output: %w", err)
	}

	return tests, nil
}

func listTestsFromProject() ([]TestInfo, error) {
	// Use ripgrep to find all Go test functions in *_test.go files only
	cmd := exec.Command("rg", "--type", "go", "-g", "*_test.go", "^func Test[A-Za-z0-9_]+\\(", "-n", "--no-heading")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ripgrep: %w", err)
	}

	return listTests(bytes.NewReader(output))
}

func extractPackagePath(filename string) string {
	// Convert filename to package path
	// e.g., "internal/pr_test.go" -> "./internal/..."
	dir := filepath.Dir(filename)
	if dir == "." || dir == "" {
		return "./"
	}
	return "./" + dir + "/..."
}

type goTest struct {
	dir string
	env []string

	flags  testFlags
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type testFlags struct {
	verbose bool
	rerun   bool
}

type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output,omitempty"`
}

type historicalCommand struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
}

func (gt goTest) run(ctx context.Context, path string, args ...string) error {
	cmd := gt.prepareCmd(ctx, path, args...)

	// Capture output while writing to stdout
	var capturedOutput bytes.Buffer
	multiWriter := io.MultiWriter(gt.stdout, &capturedOutput)
	cmd.Stdout = multiWriter

	fmt.Fprintf(gt.stdout, "💨 %s\n", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	if err != nil {
		return err
	}

	// only persist non-re-runs
	if !gt.flags.rerun {
		gt.persistCommandHistory(path, args...)
	}

	// Return the original error (test failures are expected)
	return err
}

func (gt goTest) prepareCmd(ctx context.Context, path string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{"test", path, "-count=1"}, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)

	if gt.flags.verbose {
		cmd.Args = append(cmd.Args, "-v")
	}

	cmd.Stdin = gt.stdin
	cmd.Stderr = gt.stderr
	cmd.Env = gt.env

	return cmd
}

func buildRunPattern(testNames ...string) string {
	if len(testNames) == 0 {
		return ""
	}
	if len(testNames) == 1 {
		// For single test, just use the name without anchors to allow partial matching
		return testNames[0]
	}

	pattern := "^(" + testNames[0]
	for _, test := range testNames[1:] {
		pattern += "|" + test
	}
	pattern += ")$"
	return pattern
}

var testCommandHistoryFile = os.Getenv("HOME") + "/.dev_test_history"

func (gt goTest) persistCommandHistory(path string, args ...string) {
	cmd := historicalCommand{
		Path: path,
		Args: args,
	}

	f, err := os.OpenFile(testCommandHistoryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(gt.stderr, "Warning: failed to open %s: %v\n", testCommandHistoryFile, err)
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	if err := encoder.Encode(cmd); err != nil {
		fmt.Fprintf(gt.stderr, "Warning: failed to encode command: %v\n", err)
	}
}

func (gt goTest) readCommandHistory(offset int) (string, []string, error) {
	f, err := os.Open(testCommandHistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("no historical command found")
		}
		return "", nil, err
	}
	defer f.Close()

	var commands []historicalCommand
	decoder := json.NewDecoder(f)
	for {
		var cmd historicalCommand
		if err := decoder.Decode(&cmd); err != nil {
			if err == io.EOF {
				break
			}
			continue // skip invalid entries
		}
		commands = append(commands, cmd)
	}
	if len(commands) == 0 {
		return "", nil, fmt.Errorf("no historical command found")
	}
	index := len(commands) - 1 - offset
	if index < 0 {
		return "", nil, fmt.Errorf("offset %d is out of range", offset)
	}
	cmd := commands[index]
	return cmd.Path, cmd.Args, nil
}
