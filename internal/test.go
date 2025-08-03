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

	"github.com/urfave/cli/v2"
)

var failedTestsFile = os.Getenv("HOME") + "/.dev-cli-failed-tests"

func handleTest(stdout, stderr io.Writer) cli.ActionFunc {
	goTest := goTest{
		stdin:  os.Stdin,
		stdout: stdout,
		stderr: stderr,
		env:    os.Environ(),
	}
	return func(ctx *cli.Context) error {

		shouldRunAll := ctx.Bool("all")
		shouldRunFailedOnly := ctx.Bool("failed")

		if shouldRunAll {
			return goTest.run(ctx.Context, "./...")
		}

		if shouldRunFailedOnly {
			failedTests, err := goTest.readFailedTests()
			if err != nil {
				return fmt.Errorf("failed to read failed tests: %w", err)
			}

			if len(failedTests) == 0 {
				fmt.Fprintf(stdout, "No previously failed tests found\n")
				return nil
			}

			fmt.Fprintf(stdout, "Running %d previously failed tests...\n", len(failedTests))

			testPattern := buildRunPattern(failedTests)
			return goTest.run(ctx.Context, "./...", "-run", testPattern)
		}

		return nil
	}
}

type goTest struct {
	dir string
	env []string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

type TestEvent struct {
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

		var event TestEvent
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

func buildRunPattern(testNames []string) string {
	if len(testNames) == 0 {
		return ""
	}
	if len(testNames) == 1 {
		return "^" + testNames[0] + "$"
	}

	pattern := "^(" + testNames[0]
	for _, test := range testNames[1:] {
		pattern += "|" + test
	}
	pattern += ")$"
	return pattern
}
