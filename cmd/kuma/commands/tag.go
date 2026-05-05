package commands

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

func NewTagCommand(rt Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Manage tags",
	}

	cmd.AddCommand(newTagListCmd(rt))
	cmd.AddCommand(newTagAddCmd(rt))
	cmd.AddCommand(newTagDeleteCmd(rt))

	return cmd
}

func newTagListCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tags",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runTagList(rt); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func runTagList(rt Runtime) error {
	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	tags, err := api.ListTags(ctx)
	if err != nil {
		return err
	}

	if rt.outputFormat() == "json" {
		return printJSON(tags)
	}

	w := newTableWriter()
	fmt.Fprintln(w, styleHeader.Render("ID\tNAME\tCOLOR"))
	for _, tag := range tags {
		fmt.Fprintf(w, "%d\t%s\t%s\n", tag.ID, tag.Name, tag.Color)
	}
	return w.Flush()
}

func newTagAddCmd(rt Runtime) *cobra.Command {
	var name string
	var color string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a tag",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runTagAdd(rt, name, color); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Tag name")
	cmd.Flags().StringVar(&color, "color", "", "Tag color")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("color")
	return cmd
}

func runTagAdd(rt Runtime, name, color string) error {
	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	tag, err := api.AddTag(ctx, name, color)
	if err != nil {
		return err
	}

	if rt.outputFormat() == "json" {
		return printJSON(tag)
	}

	fmt.Printf("Tag created: %d %s\n", tag.ID, tag.Name)
	return nil
}

func newTagDeleteCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a tag",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runTagDelete(rt, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func runTagDelete(rt Runtime, rawID string) error {
	tagID, err := strconv.Atoi(rawID)
	if err != nil {
		return fmt.Errorf("invalid tag ID %q", rawID)
	}

	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	if err := api.DeleteTag(ctx, tagID); err != nil {
		return err
	}

	fmt.Printf("Tag %d deleted successfully.\n", tagID)
	return nil
}
