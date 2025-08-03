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

var failedTestsFile = os.Getenv("HOME") + "/.dev-cli-failed-tests"

func handleTest(stdout, stderr io.Writer) cli.ActionFunc {
	goTest := goTest{
		stdin:  os.Stdin,
		stdout: stdout,
		stderr: stderr,
		env:    os.Environ(),
	}
	return func(ctx *cli.Context) error {
		if ctx.Bool("all") {
			return goTest.run(ctx.Context, "./...")
		}

		if ctx.Bool("failed") {
			return runFailedTests(ctx, goTest, stdout)
		}

		selectedTest, err := promptForTest()
		if err != nil {
			return err
		}

		return runSelectedTest(ctx, goTest, selectedTest)
	}
}

func runFailedTests(ctx *cli.Context, goTest goTest, stdout io.Writer) error {
	failedTests, err := goTest.readFailedTests()
	if err != nil {
		return fmt.Errorf("failed to read failed tests: %w", err)
	}

	if len(failedTests) == 0 {
		fmt.Fprintf(stdout, "No previously failed tests found\n")
		return nil
	}

	fmt.Fprintf(stdout, "Running %d previously failed tests...\n", len(failedTests))
	testPattern := buildRunPattern(failedTests...)
	return goTest.run(ctx.Context, "./...", "-run", testPattern)
}

func promptForTest() (TestInfo, error) {
	tests, err := ListTestsFromProject()
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
			uniqueName := fmt.Sprintf("\t🧪%s", test.Name)
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
	// only include the option if it includes the filter
	return strings.Contains(optValue, filterValue)
}

func ListTests(reader io.Reader) ([]TestInfo, error) {
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

func ListTestsFromProject() ([]TestInfo, error) {
	// Use ripgrep to find all Go test functions in *_test.go files only
	cmd := exec.Command("rg", "--type", "go", "-g", "*_test.go", "^func Test[A-Za-z0-9_]+\\(", "-n", "--no-heading")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ripgrep: %w", err)
	}

	return ListTests(bytes.NewReader(output))
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

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type testEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output,omitempty"`
}

func (gt goTest) run(ctx context.Context, path string, args ...string) error {
	cmd := gt.prepareCmd(ctx, path, args...)

	// Capture output while writing to stdout
	var capturedOutput bytes.Buffer
	multiWriter := io.MultiWriter(gt.stdout, &capturedOutput)
	cmd.Stdout = multiWriter

	fmt.Fprintf(gt.stdout, "💨 %s\n", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	// Process captured output through test2json even if tests failed
	failures, parseErr := gt.parseTestOutput(ctx, capturedOutput.Bytes())
	if parseErr != nil {
		// Don't fail the whole command if parsing fails
		fmt.Fprintf(gt.stderr, "Warning: failed to parse test output: %v\n", parseErr)
	}

	// Save failures for --failed flag
	if len(failures) > 0 {
		gt.saveFailedTests(failures)
	} else {
		// All tests passed, remove any existing failed tests file
		os.Remove(failedTestsFile)
	}

	// Return the original error (test failures are expected)
	return err
}

func (gt goTest) prepareCmd(ctx context.Context, path string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{"test", path, "-count=1"}, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Stdin = gt.stdin
	cmd.Stderr = gt.stderr
	cmd.Env = gt.env

	return cmd
}

func (gt goTest) parseTestOutput(ctx context.Context, output []byte) ([]string, error) {
	// Run go tool test2json on the captured output
	cmd := exec.CommandContext(ctx, "go", "tool", "test2json")
	cmd.Stdin = bytes.NewReader(output)
	cmd.Env = gt.env

	jsonOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run test2json: %w", err)
	}

	// Parse the JSON output to extract failures
	var failures []string
	scanner := bufio.NewScanner(bytes.NewReader(jsonOutput))
	for scanner.Scan() {
		line := scanner.Text()

		var event testEvent
		if json.Unmarshal([]byte(line), &event) == nil {
			if event.Action == "fail" && event.Test != "" {
				failures = append(failures, event.Test)
			}
		}
	}

	return failures, scanner.Err()
}

func (gt goTest) saveFailedTests(failures []string) {
	f, err := os.Create(failedTestsFile)
	if err != nil {
		fmt.Fprintf(gt.stderr, "Warning: failed to create %s: %v\n", failedTestsFile, err)
		return
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, failure := range failures {
		if _, err := fmt.Fprintln(writer, failure); err != nil {
			fmt.Fprintf(gt.stderr, "Warning: failed to write test failure: %v\n", err)
			return
		}
	}

	if err := writer.Flush(); err != nil {
		fmt.Fprintf(gt.stderr, "Warning: failed to flush failed tests to file: %v\n", err)
	}
}

func (gt goTest) readFailedTests() ([]string, error) {
	file, err := os.Open(failedTestsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No failed tests file exists
		}
		return nil, err
	}
	defer file.Close()

	var tests []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		test := scanner.Text()
		if test != "" {
			tests = append(tests, test)
		}
	}

	return tests, scanner.Err()
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
