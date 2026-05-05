package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jinkp/kumaapi/internal/models"
	"github.com/jinkp/kumaapi/pkg/kumaapi"
	"github.com/spf13/cobra"
)

func NewMonitorCommand(rt Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Manage monitors",
	}

	cmd.AddCommand(newMonitorListCmd(rt))
	cmd.AddCommand(newMonitorGetCmd(rt))
	cmd.AddCommand(newMonitorAddCmd(rt))
	cmd.AddCommand(newMonitorPauseCmd(rt))
	cmd.AddCommand(newMonitorResumeCmd(rt))
	cmd.AddCommand(newMonitorDeleteCmd(rt))
	cmd.AddCommand(newMonitorWatchCmd(rt))

	return cmd
}

func newMonitorListCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List monitors",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runMonitorList(rt); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func runMonitorList(rt Runtime) error {
	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	monitors, err := api.ListMonitors(ctx)
	if err != nil {
		return err
	}

	if rt.outputFormat() == "json" {
		return printJSON(monitors)
	}

	w := newTableWriter()
	fmt.Fprintln(w, styleHeader.Render("ID\tNAME\tTYPE\tSTATUS\tINTERVAL"))
	for _, monitor := range monitors {
		hb, hbErr := latestHeartbeat(ctx, api, monitor.ID)
		status := models.MonitorStatus(-1)
		if hb != nil {
			status = hb.Status
		}
		if hbErr != nil {
			status = models.MonitorStatus(-1)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%ds\n", monitor.ID, monitor.Name, monitor.Type, statusText(status), monitor.Interval)
	}
	return w.Flush()
}

func newMonitorGetCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a monitor by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runMonitorGet(rt, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func runMonitorGet(rt Runtime, rawID string) error {
	monitorID, err := strconv.Atoi(rawID)
	if err != nil {
		return fmt.Errorf("invalid monitor ID %q", rawID)
	}

	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	monitor, err := api.GetMonitor(ctx, monitorID)
	if err != nil {
		return err
	}

	hb, _ := latestHeartbeat(ctx, api, monitorID)
	if rt.outputFormat() == "json" {
		return printJSON(struct {
			Monitor          *models.Monitor   `json:"monitor"`
			CurrentStatus    string            `json:"currentStatus,omitempty"`
			CurrentHeartbeat *models.Heartbeat `json:"currentHeartbeat,omitempty"`
		}{
			Monitor: monitor,
			CurrentStatus: statusPlain(func() models.MonitorStatus {
				if hb == nil {
					return models.MonitorStatus(-1)
				}
				return hb.Status
			}()),
			CurrentHeartbeat: hb,
		})
	}

	w := newTableWriter()
	fmt.Fprintln(w, styleHeader.Render("FIELD\tVALUE"))
	fmt.Fprintf(w, "ID\t%d\n", monitor.ID)
	fmt.Fprintf(w, "Name\t%s\n", monitor.Name)
	fmt.Fprintf(w, "Type\t%s\n", monitor.Type)
	fmt.Fprintf(w, "URL\t%s\n", monitor.URL)
	fmt.Fprintf(w, "Hostname\t%s\n", monitor.Hostname)
	fmt.Fprintf(w, "Port\t%d\n", monitor.Port)
	fmt.Fprintf(w, "Method\t%s\n", monitor.Method)
	fmt.Fprintf(w, "Active\t%s\n", boolLabel(monitor.Active.IsActive()))
	fmt.Fprintf(w, "Interval\t%ds\n", monitor.Interval)
	fmt.Fprintf(w, "Retry Interval\t%ds\n", monitor.RetryInterval)
	fmt.Fprintf(w, "Max Retries\t%d\n", monitor.MaxRetries)
	fmt.Fprintf(w, "Timeout\t%ds\n", monitor.Timeout)
	fmt.Fprintf(w, "Ignore TLS\t%s\n", boolLabel(monitor.IgnoreTLS))
	fmt.Fprintf(w, "Tags\t%s\n", joinedTags(monitor.Tags))
	if hb != nil {
		fmt.Fprintf(w, "Current Status\t%s\n", statusText(hb.Status))
		fmt.Fprintf(w, "Last Heartbeat\t%s\n", heartbeatDisplayTime(hb.Time))
		fmt.Fprintf(w, "Last Result\t%s\n", heartbeatSummary(hb))
	}
	return w.Flush()
}

func newMonitorAddCmd(rt Runtime) *cobra.Command {
	var monitorType string
	var name string
	var url string
	var interval int
	var method string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a monitor",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runMonitorAdd(rt, monitorType, name, url, interval, method); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&monitorType, "type", "", "Monitor type")
	cmd.Flags().StringVar(&name, "name", "", "Monitor name")
	cmd.Flags().StringVar(&url, "url", "", "Target URL")
	cmd.Flags().IntVar(&interval, "interval", 60, "Check interval in seconds")
	cmd.Flags().StringVar(&method, "method", "GET", "HTTP method")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("url")

	return cmd
}

func runMonitorAdd(rt Runtime, monitorType, name, url string, interval int, method string) error {
	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	monitorID, err := api.AddMonitor(ctx, models.AddMonitorRequest{
		Type:               models.MonitorType(monitorType),
		Name:               name,
		URL:                url,
		Method:             method,
		Interval:           interval,
		Active:             true,
		Conditions:         []models.MonitorCondition{},
		AcceptedStatCodes:  []string{"200-299"},
		NotificationIDList: map[string]bool{},
	})
	if err != nil {
		return err
	}

	if rt.outputFormat() == "json" {
		return printJSON(map[string]any{"id": monitorID, "name": name, "type": monitorType, "url": url})
	}

	fmt.Printf("Monitor created with ID %d\n", monitorID)
	return nil
}

func newMonitorPauseCmd(rt Runtime) *cobra.Command {
	return monitorMutationCmd(rt, "pause", "Pause a monitor", func(ctx context.Context, api *kumaapi.API, id int) error {
		return api.PauseMonitor(ctx, id)
	})
}

func newMonitorResumeCmd(rt Runtime) *cobra.Command {
	return monitorMutationCmd(rt, "resume", "Resume a monitor", func(ctx context.Context, api *kumaapi.API, id int) error {
		return api.ResumeMonitor(ctx, id)
	})
}

func monitorMutationCmd(rt Runtime, use, short string, fn func(context.Context, *kumaapi.API, int) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			monitorID, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid monitor ID %q\n", args[0])
				os.Exit(1)
			}

			api, ctx, cancel, err := connect(rt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			defer cancel()
			defer api.Disconnect()

			if err := fn(ctx, api, monitorID); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Monitor %d %s successfully.\n", monitorID, strings.TrimSpace(use)+"d")
		},
	}
}

func newMonitorDeleteCmd(rt Runtime) *cobra.Command {
	var deleteChildren bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a monitor",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			monitorID, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid monitor ID %q\n", args[0])
				os.Exit(1)
			}

			api, ctx, cancel, err := connect(rt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			defer cancel()
			defer api.Disconnect()

			if err := api.DeleteMonitor(ctx, monitorID, deleteChildren); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Monitor %d deleted successfully.\n", monitorID)
		},
	}

	cmd.Flags().BoolVar(&deleteChildren, "children", false, "Delete child monitors too")
	return cmd
}

func newMonitorWatchCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "watch <id>",
		Short: "Watch monitor heartbeats in real time",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runMonitorWatch(rt, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func runMonitorWatch(rt Runtime, rawID string) error {
	monitorID, err := strconv.Atoi(rawID)
	if err != nil {
		return fmt.Errorf("invalid monitor ID %q", rawID)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	timeout := 15 * time.Second
	if rt.Timeout != nil {
		timeout = rt.Timeout()
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	kuma, err := connectWithContext(rt, requestCtx)
	if err != nil {
		return err
	}
	defer kuma.Disconnect()

	beats, stopWatch := kuma.WatchHeartbeats(ctx, monitorID)
	defer stopWatch()

	for {
		select {
		case hb, ok := <-beats:
			if !ok {
				return nil
			}
			symbol := "✗"
			if hb.Status == models.StatusUp {
				symbol = "✓"
			}
			if hb.Status == models.StatusPending {
				symbol = "•"
			}
			fmt.Printf("[%s] %s %-7s %s\n", heartbeatDisplayTime(hb.Time), symbol, statusPlain(hb.Status), heartbeatSummary(&hb))
		case <-ctx.Done():
			return nil
		}
	}
}

func connectWithContext(rt Runtime, ctx context.Context) (*kumaapi.API, error) {
	if rt.URL == nil || rt.Token == nil {
		return nil, fmt.Errorf("missing runtime configuration")
	}
	api, err := kumaapi.New(ctx, rt.URL())
	if err != nil {
		return nil, err
	}
	if err := api.LoginWithToken(ctx, rt.Token()); err != nil {
		api.Disconnect()
		return nil, err
	}
	return api, nil
}
