package pipeline

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ovh/cds/sdk"
)

func pipelineShowBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "logs",
		Short:   "cds pipeline logs <projectKey> <applicationName> <pipelineName> [envName] [buildID]",
		Long:    ``,
		Aliases: []string{"log"},
		Run:     showBuildPipeline,
	}

	return cmd
}

func showBuildPipeline(cmd *cobra.Command, args []string) {

	if len(args) < 3 || len(args) > 5 {
		sdk.Exit("Wrong usage: see %s\n", cmd.Short)
	}

	projectKey := args[0]
	appName := args[1]
	pipelineName := args[2]
	var buildNumber int
	var env string
	var err error
	if len(args) >= 4 {
		buildNumber, err = strconv.Atoi(args[3])
		if err != nil {
			// sdk.Exit("Error: buildID is not a number\n")
			// then it's the environment
			env = args[3]
			if len(args) == 5 {
				buildNumber, err = strconv.Atoi(args[4])
				if err != nil {
					sdk.Exit("Error: buildID is not a number\n")
				}
			}
		}
	}

	logChan, err := sdk.StreamPipelineBuild(projectKey, appName, pipelineName, env, buildNumber, false)
	if err != nil {
		sdk.Exit("Error: Cannot retrieve logs: %s\n", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 27, 1, 2, ' ', 0)
	titles := []string{"DATE", "ACTION", "LOG"}
	fmt.Fprintln(w, strings.Join(titles, "\t"))

	for l := range logChan {
		fmt.Fprintf(w, "%s\t%s\t%s",
			[]byte(l.Timestamp.String())[:19],
			l.Step,
			l.Value,
		)

		w.Flush()

		// Exit 1 if pipeline fail
		if l.ID == 0 && strings.Contains(l.Value, "status: Fail") {
			sdk.Exit("")
		}
	}
}