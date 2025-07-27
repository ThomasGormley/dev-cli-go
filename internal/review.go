package cli

import (
	"io"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-github/v74/github"
	"github.com/thomasgormley/dev-cli-go/internal/tui"
	"github.com/urfave/cli/v2"
)

func handlePRReview(stdout, stderr io.Writer, ghClient *github.Client) cli.ActionFunc {
	return func(c *cli.Context) error {
		if _, err := tea.LogToFile("/Users/thomasgormley/dev/dev-cli-go/debug.log", "DEBUG"); err != nil {
			log.Fatal(err)
		}
		// identifier := c.Args().First()
		p := tea.NewProgram(
			tui.NewModel(ghClient),
			tea.WithAltScreen(),
		)
		if _, err := p.Run(); err != nil {
			return err
		}

		return nil
	}
}
