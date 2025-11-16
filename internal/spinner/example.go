package spinner

import (
	"errors"
	"time"

	"github.com/thomasgormley/dev-cli-go/internal/print"
)

// ExampleUsage demonstrates how to use the spinner package
func ExampleUsage() {
	// Simple success case
	_ = With("Processing data", func() error {
		time.Sleep(1 * time.Second)
		return nil
	})

	// Error case with custom messages
	_ = With("Connecting to server", func() error {
		time.Sleep(500 * time.Millisecond)
		return errors.New("connection failed")
	}, WithFailureMessage("Could not connect to server"))

	// With custom styling
	_ = With("Downloading file", func() error {
		time.Sleep(2 * time.Second)
		return nil
	},
		WithSpinnerColor(print.ColorBlue),
		WithTextColor(print.ColorCyan),
		WithDoneSymbol('✓'),
		WithSuccessMessage("Download completed"),
	)

	// Manual control for complex operations
	pin, cancel := Start("Complex operation")
	defer cancel()

	// Update message during operation
	pin.UpdateMessage("Step 1: Initializing")
	time.Sleep(500 * time.Millisecond)

	pin.UpdateMessage("Step 2: Processing")
	time.Sleep(500 * time.Millisecond)

	pin.UpdateMessage("Step 3: Finalizing")
	time.Sleep(500 * time.Millisecond)

	pin.Stop("Complex operation completed")
}
