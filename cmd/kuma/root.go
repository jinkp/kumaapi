package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jinkp/kumaapi/cmd/kuma/commands"
	"github.com/jinkp/kumaapi/pkg/kumaapi"
	"github.com/spf13/cobra"
)

type app struct {
	config        Config
	configPath    string
	urlOverride   string
	tokenOverride string
	output        string
	timeout       int
}

func newRootCmd() *cobra.Command {
	loadedConfig, configPath, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	app := &app{
		config:     loadedConfig,
		configPath: configPath,
		output:     "table",
		timeout:    15,
	}

	cmd := &cobra.Command{
		Use:           "kuma",
		Short:         "CLI for Uptime Kuma via kumaapi",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "help" {
				return nil
			}
			switch app.outputFormat() {
			case "table", "json":
				return nil
			default:
				return fmt.Errorf("invalid output format %q (expected table|json)", app.output)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.PersistentFlags().StringVar(&app.urlOverride, "url", "", "Uptime Kuma URL (overrides config)")
	cmd.PersistentFlags().StringVar(&app.tokenOverride, "token", "", "JWT token (overrides config)")
	cmd.PersistentFlags().StringVar(&app.output, "output", "table", "Output format: table|json")
	cmd.PersistentFlags().IntVar(&app.timeout, "timeout", 15, "Request timeout in seconds")

	cmd.AddCommand(newLoginCmd(app))
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(commands.NewMonitorCommand(app.runtime()))
	cmd.AddCommand(commands.NewStatusCommand(app.runtime()))
	cmd.AddCommand(commands.NewTagCommand(app.runtime()))
	cmd.AddCommand(commands.NewAPIKeyCommand(app.runtime()))

	return cmd
}

func newLoginCmd(app *app) *cobra.Command {
	var username string
	var password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate and persist a JWT token",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runLogin(app, username, password); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&username, "user", "", "Username")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("password")

	return cmd
}

func runLogin(app *app, username, password string) error {
	url := app.effectiveURL()
	if url == "" {
		return fmt.Errorf("--url is required for login")
	}

	ctx, cancel := context.WithTimeout(context.Background(), app.timeoutDuration())
	defer cancel()

	api, err := kumaapi.New(ctx, url)
	if err != nil {
		return err
	}
	defer api.Disconnect()

	if err := api.Login(ctx, username, password); err != nil {
		return err
	}

	if err := saveConfig(Config{URL: url, Token: api.AuthToken()}); err != nil {
		return err
	}

	app.config.URL = url
	app.config.Token = api.AuthToken()

	fmt.Println("Logged in successfully. Token saved.")
	return nil
}

func (a *app) effectiveURL() string {
	if strings.TrimSpace(a.urlOverride) != "" {
		return strings.TrimSpace(a.urlOverride)
	}
	return strings.TrimSpace(a.config.URL)
}

func (a *app) effectiveToken() string {
	if strings.TrimSpace(a.tokenOverride) != "" {
		return strings.TrimSpace(a.tokenOverride)
	}
	return strings.TrimSpace(a.config.Token)
}

func (a *app) outputFormat() string {
	return strings.ToLower(strings.TrimSpace(a.output))
}

func (a *app) timeoutDuration() time.Duration {
	seconds := a.timeout
	if seconds <= 0 {
		seconds = 15
	}
	return time.Duration(seconds) * time.Second
}

func (a *app) runtime() commands.Runtime {
	return commands.Runtime{
		URL:     func() string { return a.effectiveURL() },
		Token:   func() string { return a.effectiveToken() },
		Output:  func() string { return a.outputFormat() },
		Timeout: func() time.Duration { return a.timeoutDuration() },
	}
}
