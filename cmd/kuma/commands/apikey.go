package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jinkp/kumaapi/internal/models"
	"github.com/jinkp/kumaapi/pkg/kumaapi"
	"github.com/spf13/cobra"
)

func NewAPIKeyCommand(rt Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikey",
		Short: "Manage API keys",
	}

	cmd.AddCommand(newAPIKeyListCmd(rt))
	cmd.AddCommand(newAPIKeyAddCmd(rt))
	cmd.AddCommand(newAPIKeyDeleteCmd(rt))
	cmd.AddCommand(newAPIKeyEnableCmd(rt))
	cmd.AddCommand(newAPIKeyDisableCmd(rt))

	return cmd
}

func newAPIKeyListCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runAPIKeyList(rt); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func runAPIKeyList(rt Runtime) error {
	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	keys, err := api.ListAPIKeys(ctx)
	if err != nil {
		return err
	}

	if rt.outputFormat() == "json" {
		return printJSON(keys)
	}

	w := newTableWriter()
	fmt.Fprintln(w, styleHeader.Render("ID\tNAME\tACTIVE\tCREATED\tEXPIRES"))
	for _, key := range keys {
		expires := ""
		if key.Expires != nil {
			expires = *key.Expires
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", key.ID, key.Name, boolLabel(key.Active.IsActive()), key.CreatedAt, expires)
	}
	return w.Flush()
}

func newAPIKeyAddCmd(rt Runtime) *cobra.Command {
	var name string
	var expires string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an API key",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runAPIKeyAdd(rt, name, expires); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "API key name")
	cmd.Flags().StringVar(&expires, "expires", "", "Expiration timestamp")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func runAPIKeyAdd(rt Runtime, name, expires string) error {
	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	var expiresPtr *string
	if expires != "" {
		expiresPtr = &expires
	}

	resp, err := api.AddAPIKey(ctx, models.AddAPIKeyRequest{Name: name, Expires: expiresPtr})
	if err != nil {
		return err
	}

	if rt.outputFormat() == "json" {
		return printJSON(resp)
	}

	fmt.Printf("API key created: id=%d key=%s\n", resp.KeyID, resp.Key)
	return nil
}

func newAPIKeyDeleteCmd(rt Runtime) *cobra.Command {
	return apiKeyMutationCmd(rt, "delete", "Delete an API key", func(api *kumaapi.API, ctx context.Context, id int) error {
		return api.DeleteAPIKey(ctx, id)
	})
}

func newAPIKeyEnableCmd(rt Runtime) *cobra.Command {
	return apiKeyMutationCmd(rt, "enable", "Enable an API key", func(api *kumaapi.API, ctx context.Context, id int) error {
		return api.EnableAPIKey(ctx, id)
	})
}

func newAPIKeyDisableCmd(rt Runtime) *cobra.Command {
	return apiKeyMutationCmd(rt, "disable", "Disable an API key", func(api *kumaapi.API, ctx context.Context, id int) error {
		return api.DisableAPIKey(ctx, id)
	})
}

func apiKeyMutationCmd(rt Runtime, use, short string, fn func(*kumaapi.API, context.Context, int) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			keyID, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid API key ID %q\n", args[0])
				os.Exit(1)
			}

			api, ctx, cancel, err := connect(rt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			defer cancel()
			defer api.Disconnect()

			if err := fn(api, ctx, keyID); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("API key %d %s successfully.\n", keyID, strings.TrimSpace(use)+"d")
		},
	}
}
