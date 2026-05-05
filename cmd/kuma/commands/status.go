package commands

import (
	"fmt"
	"os"

	"github.com/jinkp/kumaapi/internal/models"
	"github.com/spf13/cobra"
)

func NewStatusCommand(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show a system summary",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runStatus(rt); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
}

func runStatus(rt Runtime) error {
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

	info, _ := api.GetInfo(ctx)
	up := 0
	down := 0
	pending := 0

	for _, monitor := range monitors {
		hb, hbErr := latestHeartbeat(ctx, api, monitor.ID)
		if hbErr != nil || hb == nil {
			pending++
			continue
		}
		switch hb.Status {
		case models.StatusUp:
			up++
		case models.StatusDown:
			down++
		default:
			pending++
		}
	}

	if rt.outputFormat() == "json" {
		version := ""
		timezone := ""
		if info != nil {
			version = info.Version
			timezone = info.Timezone
		}
		return printJSON(map[string]any{
			"version":  version,
			"timezone": timezone,
			"monitors": map[string]int{
				"total":   len(monitors),
				"up":      up,
				"down":    down,
				"pending": pending,
			},
		})
	}

	versionLine := "Uptime Kuma"
	if info != nil && info.Version != "" {
		versionLine = "Uptime Kuma " + info.Version
	}
	if info != nil && info.Timezone != "" {
		versionLine += " — " + info.Timezone
	}

	fmt.Println(versionLine)
	fmt.Printf("Monitors: %d total, %d UP, %d DOWN", len(monitors), up, down)
	if pending > 0 {
		fmt.Printf(", %d PENDING", pending)
	}
	fmt.Println()
	return nil
}
