package commands

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func NewTasks(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use: "tasks", Short: "Manage the shared Kanban task board",
		Long: "Manage account and event tasks. Use people to discover assignee membership IDs.\nUse show to read lock_version before editing, moving, archiving or restoring.\nTask text records work to do; completing a task does not execute that work.",
	}
	command.AddCommand(newTasksList(runtime), newTasksShow(runtime), newTasksPeople(runtime),
		newTasksSave(runtime, true), newTasksSave(runtime, false), newTasksMove(runtime),
		newTasksComments(runtime), newTasksComment(runtime),
		newTasksArchive(runtime, false), newTasksArchive(runtime, true), newTasksDelete(runtime))
	return command
}

func newTasksList(runtime *appctx.Runtime) *cobra.Command {
	var event, assignee string
	var archived, hideDone bool
	command := &cobra.Command{
		Use: "list", Short: "List tasks in board order", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			filters := url.Values{}
			if event != "" {
				filters.Set("event_slug", event)
			}
			if assignee != "" {
				filters.Set("assignee_id", assignee)
			}
			if archived {
				filters.Set("archived", "true")
			}
			if hideDone {
				filters.Set("hide_done", "true")
			}
			response, err := client.ListTasks(command.Context(), filters)
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs || runtime.OutputFormat() == output.FormatCount {
				data = response.Tasks
			}
			return runtime.Output().OK(data, renderTasks(response.Tasks), output.WithSummary(summaryCount(len(response.Tasks), "task", "tasks")))
		},
	}
	command.Flags().StringVar(&event, "event", "", "event slug, or none for account-wide tasks")
	command.Flags().StringVar(&assignee, "assignee", "", "membership ID, me, or none")
	command.Flags().BoolVar(&archived, "archived", false, "show the archive instead of the active board")
	command.Flags().BoolVar(&hideDone, "hide-done", false, "hide completed tasks on the active board")
	return command
}

func newTasksShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{Use: "show ID", Short: "Show a task and its current version", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			task, err := client.GetTask(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(task, renderTask(task))
		},
	}
}

func newTasksPeople(runtime *appctx.Runtime) *cobra.Command {
	var event string
	command := &cobra.Command{Use: "people", Short: "List eligible assignee membership IDs", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.TaskPeople(command.Context(), event)
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs || runtime.OutputFormat() == output.FormatCount {
				data = response.People
			}
			return runtime.Output().OK(data, func(destination io.Writer) error {
				for _, person := range response.People {
					if _, err := fmt.Fprintf(destination, "%d  %s\n", person.ID, terminal.SanitizeLine(person.Name)); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	command.Flags().StringVar(&event, "event", "", "event slug; omit for account-wide assignments")
	return command
}

func newTasksSave(runtime *appctx.Runtime, create bool) *cobra.Command {
	var title, description, status, event, assignee, due string
	var version int
	use, short := "update ID", "Update selected task fields"
	args := cobra.ExactArgs(1)
	if create {
		use, short, args = "create", "Create a task", cobra.NoArgs
	}
	command := &cobra.Command{Use: use, Short: short, Args: args}
	command.RunE = func(command *cobra.Command, args []string) error {
		attributes := map[string]any{}
		for flag, value := range map[string]string{"title": title, "description": description, "status": status, "event": event, "assignee": assignee, "due": due} {
			if command.Flags().Changed(flag) {
				key := flag
				switch flag {
				case "event":
					key = "event_slug"
				case "assignee":
					key = "assignee_id"
				case "due":
					key = "due_on"
				}
				attributes[key] = value
			}
		}
		if create && strings.TrimSpace(title) == "" {
			return output.ErrUsage("--title is required")
		}
		if !create && len(attributes) == 0 {
			return output.ErrUsage("provide at least one field to update")
		}
		if command.Flags().Changed("status") && !validTaskStatus(status) {
			return output.ErrUsage("--status must be open, in_progress, or done")
		}
		if err := taskVersion(command, attributes, version); err != nil {
			return err
		}
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		if create {
			task, location, err := client.CreateTask(command.Context(), attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(task, renderTask(task), output.WithMeta("location", location))
		}
		task, err := client.UpdateTask(command.Context(), args[0], attributes)
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(task, renderTask(task))
	}
	command.Flags().StringVar(&title, "title", "", "task title")
	command.Flags().StringVar(&description, "description", "", "replace description (HTML allowed); empty string clears it, including files")
	command.Flags().StringVar(&status, "status", "", "open, in_progress, or done")
	command.Flags().StringVar(&event, "event", "", "event slug; empty string clears the event")
	command.Flags().StringVar(&assignee, "assignee", "", "membership ID from people; empty string unassigns")
	command.Flags().StringVar(&due, "due", "", "due date YYYY-MM-DD; empty string clears it")
	if !create {
		command.Flags().IntVar(&version, "lock-version", 0, "version from show to detect concurrent changes")
	}
	return command
}

func newTasksMove(runtime *appctx.Runtime) *cobra.Command {
	var status, before string
	var version int
	command := &cobra.Command{Use: "move ID", Short: "Move a task to a column or reorder it", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !validTaskStatus(status) {
				return output.ErrUsage("--status must be open, in_progress, or done")
			}
			attributes := map[string]any{"status": status}
			if before != "" {
				attributes["before_id"] = before
			}
			if err := taskVersion(command, attributes, version); err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			task, err := client.MoveTask(command.Context(), args[0], attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(task, renderTask(task))
		},
	}
	command.Flags().StringVar(&status, "status", "", "destination: open, in_progress, or done (required)")
	command.Flags().StringVar(&before, "before", "", "task ID in destination column; omit to place last")
	command.Flags().IntVar(&version, "lock-version", 0, "version from show to detect concurrent changes")
	return command
}

func newTasksComments(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{Use: "comments ID", Short: "Read comments and task activity", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.TaskEntries(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs || runtime.OutputFormat() == output.FormatCount {
				data = response.Entries
			}
			return runtime.Output().OK(data, func(destination io.Writer) error {
				for _, entry := range response.Entries {
					if err := renderTaskEntry(entry)(destination); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
}

func newTasksComment(runtime *appctx.Runtime) *cobra.Command {
	var body string
	command := &cobra.Command{Use: "comment ID", Short: "Add a comment to a task", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if strings.TrimSpace(body) == "" {
				return output.ErrUsage("--body is required")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			entry, err := client.CommentOnTask(command.Context(), args[0], body)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(entry, renderTaskEntry(entry))
		},
	}
	command.Flags().StringVar(&body, "body", "", "comment text or HTML (required)")
	return command
}

func newTasksArchive(runtime *appctx.Runtime, restore bool) *cobra.Command {
	var version int
	use, short := "archive ID", "Archive a completed task"
	if restore {
		use, short = "restore ID", "Restore an archived task to Done"
	}
	command := &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			attributes := map[string]any{}
			if err := taskVersion(command, attributes, version); err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var task api.Task
			if restore {
				task, err = client.RestoreTask(command.Context(), args[0], attributes)
			} else {
				task, err = client.ArchiveTask(command.Context(), args[0], attributes)
			}
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(task, renderTask(task))
		},
	}
	command.Flags().IntVar(&version, "lock-version", 0, "version from show to detect concurrent changes")
	return command
}

func newTasksDelete(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete ID", Short: "Delete a task and its comments and history", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("deleting a task requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.DeleteTask(command.Context(), args[0]); err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(map[string]string{"id": args[0], "status": "deleted"}, func(destination io.Writer) error {
				_, err := fmt.Fprintf(destination, "Deleted task %s\n", terminal.SanitizeLine(args[0]))
				return err
			})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm permanent deletion")
	return command
}

func taskVersion(command *cobra.Command, attributes map[string]any, version int) error {
	if command.Flags().Changed("lock-version") {
		if version < 0 {
			return output.ErrUsage("--lock-version must be zero or greater")
		}
		attributes["lock_version"] = version
	}
	return nil
}

func validTaskStatus(status string) bool {
	return status == "open" || status == "in_progress" || status == "done"
}

func renderTasks(tasks []api.Task) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(tasks) == 0 {
			_, err := fmt.Fprintln(destination, "No tasks.")
			return err
		}
		table := tabwriter.NewWriter(destination, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "ID\tSTATUS\tTITLE\tASSIGNEE\tDUE\tVERSION"); err != nil {
			return err
		}
		for _, task := range tasks {
			person, due := "—", "—"
			if task.Assignee != nil {
				person = terminal.SanitizeLine(task.Assignee.Name)
			}
			if task.DueOn != nil {
				due = terminal.SanitizeLine(*task.DueOn)
			}
			if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%d\n", terminal.SanitizeLine(task.ID), terminal.SanitizeLine(task.Status), terminal.SanitizeLine(task.Title), person, due, task.LockVersion); err != nil {
				return err
			}
		}
		return table.Flush()
	}
}

func renderTask(task api.Task) output.StyledRenderer {
	return func(destination io.Writer) error {
		if err := renderTasks([]api.Task{task})(destination); err != nil {
			return err
		}
		if task.Event != nil {
			if _, err := fmt.Fprintf(destination, "Event: %s (%s)\n", terminal.SanitizeLine(task.Event.Title), terminal.SanitizeLine(task.Event.Slug)); err != nil {
				return err
			}
		}
		if task.ArchivedAt != nil {
			if _, err := fmt.Fprintf(destination, "Archived: %s\n", terminal.SanitizeLine(*task.ArchivedAt)); err != nil {
				return err
			}
		}
		if task.Description != "" {
			if _, err := fmt.Fprintln(destination, terminal.SanitizeLine(task.Description)); err != nil {
				return err
			}
		}
		return renderTaskFiles(destination, task.Attachments)
	}
}

func renderTaskEntry(entry api.TaskEntry) output.StyledRenderer {
	return func(destination io.Writer) error {
		author, body := "Usetix", ""
		if entry.Author != nil {
			author = terminal.SanitizeLine(*entry.Author)
		}
		if entry.Body != nil {
			body = terminal.SanitizeLine(*entry.Body)
		}
		if entry.Kind != "comment" {
			body = terminal.SanitizeLine(fmt.Sprint(entry.Details))
		}
		if _, err := fmt.Fprintf(destination, "%s  %s · %s\n%s\n", terminal.SanitizeLine(entry.CreatedAt), author, terminal.SanitizeLine(entry.Kind), body); err != nil {
			return err
		}
		return renderTaskFiles(destination, entry.Attachments)
	}
}

func renderTaskFiles(destination io.Writer, files []api.TaskAttachment) error {
	for _, file := range files {
		if _, err := fmt.Fprintf(destination, "File: %s (%d bytes)\n  %s\n", terminal.SanitizeLine(file.Filename), file.ByteSize, terminal.SanitizeLine(file.URL)); err != nil {
			return err
		}
	}
	return nil
}
