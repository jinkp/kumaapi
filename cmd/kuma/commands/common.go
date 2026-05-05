package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jinkp/kumaapi/internal/models"
	"github.com/jinkp/kumaapi/pkg/kumaapi"
)

var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleUp     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleDown   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleDim    = lipgloss.NewStyle().Faint(true)
)

type Runtime struct {
	URL     func() string
	Token   func() string
	Output  func() string
	Timeout func() time.Duration
}

func (r Runtime) context() (context.Context, context.CancelFunc) {
	timeout := 15 * time.Second
	if r.Timeout != nil {
		if v := r.Timeout(); v > 0 {
			timeout = v
		}
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (r Runtime) outputFormat() string {
	if r.Output == nil {
		return "table"
	}
	value := strings.TrimSpace(strings.ToLower(r.Output()))
	if value == "" {
		return "table"
	}
	return value
}

func connect(rt Runtime) (*kumaapi.API, context.Context, context.CancelFunc, error) {
	if rt.URL == nil || strings.TrimSpace(rt.URL()) == "" {
		return nil, nil, nil, fmt.Errorf("missing Uptime Kuma URL; pass --url or run kuma login")
	}
	if rt.Token == nil || strings.TrimSpace(rt.Token()) == "" {
		return nil, nil, nil, fmt.Errorf("missing JWT token; run kuma login or pass --token")
	}

	ctx, cancel := rt.context()
	api, err := kumaapi.New(ctx, strings.TrimSpace(rt.URL()))
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	if err := api.LoginWithToken(ctx, strings.TrimSpace(rt.Token())); err != nil {
		api.Disconnect()
		cancel()
		return nil, nil, nil, err
	}
	return api, ctx, cancel, nil
}

func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func newTableWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

func statusText(status models.MonitorStatus) string {
	switch status {
	case models.StatusUp:
		return styleUp.Render("UP")
	case models.StatusDown:
		return styleDown.Render("DOWN")
	case models.StatusPending:
		return styleWarn.Render("PENDING")
	case models.StatusMaintenance:
		return styleDim.Render("MAINTENANCE")
	default:
		return styleDim.Render("UNKNOWN")
	}
}

func statusPlain(status models.MonitorStatus) string {
	switch status {
	case models.StatusUp:
		return "UP"
	case models.StatusDown:
		return "DOWN"
	case models.StatusPending:
		return "PENDING"
	case models.StatusMaintenance:
		return "MAINTENANCE"
	default:
		return "UNKNOWN"
	}
}

func latestHeartbeat(ctx context.Context, api *kumaapi.API, monitorID int) (*models.Heartbeat, error) {
	beats, err := api.GetHeartbeats(ctx, monitorID)
	if err != nil {
		return nil, err
	}
	if len(beats) == 0 {
		return nil, nil
	}

	sort.SliceStable(beats, func(i, j int) bool {
		return heartbeatTime(beats[i]).Before(heartbeatTime(beats[j]))
	})

	last := beats[len(beats)-1]
	return &last, nil
}

func heartbeatTime(hb models.Heartbeat) time.Time {
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05.000Z07:00"}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, hb.Time); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func heartbeatDisplayTime(value string) string {
	ts := heartbeatTime(models.Heartbeat{Time: value})
	if ts.IsZero() {
		return value
	}
	return ts.Local().Format("2006-01-02 15:04:05")
}

func heartbeatSummary(hb *models.Heartbeat) string {
	if hb == nil {
		return "unknown"
	}
	if hb.Ping > 0 {
		return fmt.Sprintf("%dms", hb.Ping)
	}
	if strings.TrimSpace(hb.Msg) != "" {
		return hb.Msg
	}
	if hb.Duration > 0 {
		return fmt.Sprintf("%dms", hb.Duration)
	}
	return "n/a"
}

func boolLabel(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func joinedTags(tags []models.MonitorTag) string {
	if len(tags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tags))
	for _, tag := range tags {
		parts = append(parts, tag.Name)
	}
	return strings.Join(parts, ", ")
}
