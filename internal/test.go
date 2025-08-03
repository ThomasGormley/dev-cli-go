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

func handleTest(stdout, stderr io.Writer) cli.ActionFunc {
	goTest := goTest{
		stdin:  os.Stdin,
		stdout: stdout,
		stderr: stderr,
		env:    os.Environ(),
	}
	return func(ctx *cli.Context) error {

		shouldRunAll := ctx.Bool("all")
		shouldRunFailed := ctx.Bool("failed")

		if shouldRunAll {
			return goTest.run(ctx.Context, "./...")
		}

		if shouldRunFailed {
			return goTest.runFailed(ctx.Context)
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

func (gt goTest) run(ctx context.Context, path string) error {
	cmd := gt.prepareCmd(ctx, path)

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
	f, err := os.Create(".dev-cli-failed-tests")
	if err != nil {
		fmt.Fprintf(gt.stderr, "Warning: failed to create .dev-cli-failed-tests file: %v\n", err)
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

func (gt goTest) runFailed(ctx context.Context) error {
	failedTests, err := gt.readFailedTests()
	if err != nil {
		return fmt.Errorf("failed to read failed tests: %w", err)
	}

	if len(failedTests) == 0 {
		fmt.Fprintf(gt.stdout, "No previously failed tests found\n")
		return nil
	}

	fmt.Fprintf(gt.stdout, "Running %d previously failed tests...\n", len(failedTests))

	// Build test pattern to run all failed tests at once
	testPattern := "^(" + failedTests[0]
	for _, test := range failedTests[1:] {
		testPattern += "|" + test
	}
	testPattern += ")$"

	cmd := gt.prepareCmd(ctx, "./...", "-run", testPattern)

	// Capture output while writing to stdout
	var capturedOutput bytes.Buffer
	multiWriter := io.MultiWriter(gt.stdout, &capturedOutput)
	cmd.Stdout = multiWriter

	err = cmd.Run()

	// Process captured output through test2json to find which tests still fail
	stillFailing, parseErr := gt.parseTestOutput(ctx, capturedOutput.Bytes())
	if parseErr != nil {
		fmt.Fprintf(gt.stderr, "Warning: failed to parse test output: %v\n", parseErr)
		// If we can't parse, keep all the original failed tests
		stillFailing = failedTests
	}

	// Update the failed tests file with only the tests that are still failing
	if len(stillFailing) > 0 {
		gt.saveFailedTests(stillFailing)
		fmt.Fprintf(gt.stdout, "%d tests still failing\n", len(stillFailing))
	} else {
		// All tests passed, remove the failed tests file
		os.Remove(".dev-cli-failed-tests")
		fmt.Fprintf(gt.stdout, "All previously failed tests now pass!\n")
	}

	return err
}

func (gt goTest) readFailedTests() ([]string, error) {
	file, err := os.Open(".dev-cli-failed-tests")
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
