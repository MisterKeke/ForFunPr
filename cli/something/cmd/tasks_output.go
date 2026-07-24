package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"currency-wails/cli/internal/apiclient"
)

func printTasks(output io.Writer, tasks []apiclient.Task) {
	if len(tasks) == 0 {
		fmt.Fprintln(output, "No tasks found.")
		return
	}

	table := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tDONE\tDUE\tPRIORITY\tTITLE")

	for _, task := range tasks {
		done := "no"
		if task.Done {
			done = "yes"
		}

		dueDate := task.DueDate
		if dueDate == "" {
			dueDate = "-"
		}

		priority := task.Priority
		if priority == "" {
			priority = "-"
		}

		fmt.Fprintf(
			table,
			"%d\t%s\t%s\t%s\t%s\n",
			task.ID,
			done,
			dueDate,
			priority,
			task.Title,
		)
	}

	_ = table.Flush()
}
