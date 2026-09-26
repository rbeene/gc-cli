package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func Execute() int {
	app, err := NewApp(os.Stdout, os.Stderr)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to initialize app:", err)
		return 1
	}
	defer app.Close()

	cmd := NewRootCmd(app)
	if err := cmd.Execute(); err != nil {
		_ = app.Printer.PrintError(err)
		return 1
	}
	return 0
}

func NewRootCmd(app *App) *cobra.Command {
	var jsonOutput bool
	useName := filepath.Base(os.Args[0])
	if useName == "" {
		useName = "gc-cli"
	}
	cmd := &cobra.Command{
		Use:     useName,
		Short:   "Google Classroom CLI",
		Long:    "A Google Classroom terminal client with API-first parity and seamless web handoff.",
		Example: "  gc auth login\n  gc classes list\n  gc classes show <course_id>\n  gc submissions turn-in <course_id> <course_work_id> <submission_id>\n  gc todo",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			app.SetJSONOutput(jsonOutput)
			return guardReadOnly(cmd)
		},
		SilenceUsage: true,
		// Execute already renders errors through app.Printer, which is the only
		// path that honours --json. Letting cobra print them too emitted every
		// error twice and put non-JSON text on the JSON stream.
		SilenceErrors: true,
	}
	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output machine-readable JSON")

	cmd.AddCommand(
		newAuthCmd(app),
		newClassesCmd(app),
		newStreamCmd(app),
		newClassworkCmd(app),
		newMaterialsCmd(app),
		newSubmissionsCmd(app),
		newPeopleCmd(app),
		newGradesCmd(app),
		newTopicsCmd(app),
		newTodoCmd(app),
		newCalendarCmd(app),
		newHandoffCmd(app),
	)
	return cmd
}
