package commands

import (
	"context"
	"encoding/json"
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
	cmd.AddCommand(newMonitorEditCmd(rt))
	cmd.AddCommand(newMonitorTypesCmd(rt))
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

type commonMonitorFlags struct {
	name          string
	interval      int
	retryInterval int
	maxRetries    int
	timeout       int
	paused        bool
	parent        int
}

func addCommonMonitorFlags(cmd *cobra.Command, f *commonMonitorFlags) {
	cmd.Flags().StringVar(&f.name, "name", "", "Monitor name (required)")
	cmd.Flags().IntVar(&f.interval, "interval", 60, "Check interval in seconds")
	cmd.Flags().IntVar(&f.retryInterval, "retry-interval", 60, "Retry interval in seconds")
	cmd.Flags().IntVar(&f.maxRetries, "max-retries", 1, "Max retries before DOWN")
	cmd.Flags().IntVar(&f.timeout, "timeout", 48, "Timeout in seconds")
	cmd.Flags().BoolVar(&f.paused, "paused", false, "Create monitor in paused state")
	cmd.Flags().IntVar(&f.parent, "parent", 0, "Parent group monitor ID (0 = none)")
	_ = cmd.MarkFlagRequired("name")
}

func (f *commonMonitorFlags) toRequest() models.AddMonitorRequest {
	req := models.AddMonitorRequest{
		Name:               f.name,
		Interval:           f.interval,
		RetryInterval:      f.retryInterval,
		MaxRetries:         f.maxRetries,
		Timeout:            f.timeout,
		Active:             !f.paused,
		NotificationIDList: map[string]bool{},
		Conditions:         []models.MonitorCondition{},
		AcceptedStatCodes:  []string{"200-299"},
	}
	if f.parent > 0 {
		req.ParentID = &f.parent
	}
	return req
}

type httpMonitorFlags struct {
	url           string
	method        string
	body          string
	headers       string
	ignoreTLS     bool
	basicAuthUser string
	basicAuthPass string
	acceptedCodes string
	maxRedirects  int
}

type keywordMonitorFlags struct {
	httpMonitorFlags
	keyword       string
	invertKeyword bool
}

type hostnameFlags struct {
	hostname string
}

type portMonitorFlags struct {
	hostname string
	port     int
}

type dnsMonitorFlags struct {
	hostname    string
	resolveType string
	dnsServer   string
}

type dockerMonitorFlags struct {
	container  string
	dockerHost int
}

type smtpMonitorFlags struct {
	hostname     string
	port         int
	smtpSecurity string
	username     string
	password     string
	from         string
	to           string
}

type databaseMonitorFlags struct {
	hostname string
	port     int
	dbName   string
	username string
	password string
	query    string
}

func addHTTPMonitorFlags(cmd *cobra.Command, f *httpMonitorFlags) {
	cmd.Flags().StringVar(&f.url, "url", "", "Target URL (required)")
	cmd.Flags().StringVar(&f.method, "method", "GET", "HTTP method GET|POST|PUT|PATCH|DELETE")
	cmd.Flags().StringVar(&f.body, "body", "", "Request body (for POST/PUT/PATCH)")
	cmd.Flags().StringVar(&f.headers, "headers", "", `JSON headers, e.g. '{"Authorization":"Bearer token"}'`)
	cmd.Flags().BoolVar(&f.ignoreTLS, "ignore-tls", false, "Ignore TLS/SSL errors")
	cmd.Flags().StringVar(&f.basicAuthUser, "basic-auth-user", "", "Basic auth username")
	cmd.Flags().StringVar(&f.basicAuthPass, "basic-auth-pass", "", "Basic auth password")
	cmd.Flags().StringVar(&f.acceptedCodes, "accepted-codes", "200-299", "Comma-separated, e.g. '200-299,404'")
	cmd.Flags().IntVar(&f.maxRedirects, "max-redirects", 10, "Max redirects")
	_ = cmd.MarkFlagRequired("url")
}

func addHostnameFlags(cmd *cobra.Command, f *hostnameFlags) {
	cmd.Flags().StringVar(&f.hostname, "hostname", "", "Hostname or IP (required)")
	_ = cmd.MarkFlagRequired("hostname")
}

func addPortMonitorFlags(cmd *cobra.Command, f *portMonitorFlags) {
	cmd.Flags().StringVar(&f.hostname, "hostname", "", "Hostname or IP (required)")
	cmd.Flags().IntVar(&f.port, "port", 0, "TCP port number (required)")
	_ = cmd.MarkFlagRequired("hostname")
	_ = cmd.MarkFlagRequired("port")
}

func addDNSMonitorFlags(cmd *cobra.Command, f *dnsMonitorFlags) {
	cmd.Flags().StringVar(&f.hostname, "hostname", "", "Domain to resolve (required)")
	cmd.Flags().StringVar(&f.resolveType, "resolve-type", "A", "Record type: A|AAAA|CAA|CNAME|MX|NS|PTR|SOA|SRV|TXT")
	cmd.Flags().StringVar(&f.dnsServer, "dns-server", "", "Custom DNS resolver (default: system)")
	_ = cmd.MarkFlagRequired("hostname")
}

func addDockerMonitorFlags(cmd *cobra.Command, f *dockerMonitorFlags) {
	cmd.Flags().StringVar(&f.container, "container", "", "Container name or ID (required)")
	cmd.Flags().IntVar(&f.dockerHost, "docker-host", 0, "Docker host ID")
	_ = cmd.MarkFlagRequired("container")
}

func addSMTPMonitorFlags(cmd *cobra.Command, f *smtpMonitorFlags) {
	cmd.Flags().StringVar(&f.hostname, "hostname", "", "SMTP server hostname (required)")
	cmd.Flags().IntVar(&f.port, "port", 587, "SMTP port")
	cmd.Flags().StringVar(&f.smtpSecurity, "smtp-security", "starttls", "TLS security: none|starttls|tls")
	cmd.Flags().StringVar(&f.username, "smtp-username", "", "SMTP username")
	cmd.Flags().StringVar(&f.password, "smtp-password", "", "SMTP password")
	cmd.Flags().StringVar(&f.from, "smtp-from", "", "From email address")
	cmd.Flags().StringVar(&f.to, "smtp-to", "", "To email address (for test)")
	_ = cmd.MarkFlagRequired("hostname")
}

func addDatabaseMonitorFlags(cmd *cobra.Command, f *databaseMonitorFlags, defaultPort int) {
	cmd.Flags().StringVar(&f.hostname, "hostname", "", "DB server hostname (required)")
	cmd.Flags().IntVar(&f.port, "port", defaultPort, "Port")
	cmd.Flags().StringVar(&f.dbName, "db-name", "", "Database name")
	cmd.Flags().StringVar(&f.username, "username", "", "DB username")
	cmd.Flags().StringVar(&f.password, "password", "", "DB password")
	cmd.Flags().StringVar(&f.query, "query", "", "Optional SQL query to execute as health check")
	_ = cmd.MarkFlagRequired("hostname")
}

func newMonitorAddCmd(rt Runtime) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <type>",
		Short: "Add a monitor (use 'add <type> --help' for type-specific options)",
	}
	cmd.AddCommand(newMonitorAddHTTPCmd(rt))
	cmd.AddCommand(newMonitorAddKeywordCmd(rt))
	cmd.AddCommand(newMonitorAddPingCmd(rt))
	cmd.AddCommand(newMonitorAddPortCmd(rt))
	cmd.AddCommand(newMonitorAddDNSCmd(rt))
	cmd.AddCommand(newMonitorAddPushCmd(rt))
	cmd.AddCommand(newMonitorAddDockerCmd(rt))
	cmd.AddCommand(newMonitorAddSMTPCmd(rt))
	cmd.AddCommand(newMonitorAddPostgresCmd(rt))
	cmd.AddCommand(newMonitorAddMySQLCmd(rt))
	cmd.AddCommand(newMonitorAddSQLServerCmd(rt))
	return cmd
}

func newMonitorAddHTTPCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags
	var httpFlags httpMonitorFlags

	cmd := &cobra.Command{
		Use:   "http",
		Short: "Add an HTTP/HTTPS monitor",
		Run: func(cmd *cobra.Command, args []string) {
			req, err := buildHTTPAddRequest(models.MonitorTypeHTTP, common, httpFlags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addHTTPMonitorFlags(cmd, &httpFlags)
	return cmd
}

func newMonitorAddKeywordCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags
	var flags keywordMonitorFlags

	cmd := &cobra.Command{
		Use:   "keyword",
		Short: "Add an HTTP keyword monitor",
		Run: func(cmd *cobra.Command, args []string) {
			req, err := buildHTTPAddRequest(models.MonitorTypeKeyword, common, flags.httpMonitorFlags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			req.Keyword = strings.TrimSpace(flags.keyword)
			req.InvertKeyword = flags.invertKeyword
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addHTTPMonitorFlags(cmd, &flags.httpMonitorFlags)
	cmd.Flags().StringVar(&flags.keyword, "keyword", "", "Keyword to search in response (required)")
	cmd.Flags().BoolVar(&flags.invertKeyword, "invert-keyword", false, "Alert when keyword IS found")
	_ = cmd.MarkFlagRequired("keyword")
	return cmd
}

func newMonitorAddPingCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags
	var flags hostnameFlags

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Add an ICMP ping monitor",
		Run: func(cmd *cobra.Command, args []string) {
			req := common.toRequest()
			req.Type = models.MonitorTypePing
			req.Hostname = strings.TrimSpace(flags.hostname)
			if err := validateHostname(req.Hostname); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addHostnameFlags(cmd, &flags)
	return cmd
}

func newMonitorAddPortCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags
	var flags portMonitorFlags

	cmd := &cobra.Command{
		Use:   "port",
		Short: "Add a TCP port monitor",
		Run: func(cmd *cobra.Command, args []string) {
			req := common.toRequest()
			req.Type = models.MonitorTypePort
			req.Hostname = strings.TrimSpace(flags.hostname)
			req.Port = flags.port
			if err := validateHostname(req.Hostname); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := validatePort(req.Port); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addPortMonitorFlags(cmd, &flags)
	return cmd
}

func newMonitorAddDNSCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags
	var flags dnsMonitorFlags

	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Add a DNS resolution monitor",
		Run: func(cmd *cobra.Command, args []string) {
			resolveType, err := validateDNSResolveType(flags.resolveType)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			req := common.toRequest()
			req.Type = models.MonitorTypeDNS
			req.Hostname = strings.TrimSpace(flags.hostname)
			req.DNSResolveType = resolveType
			req.DNSResolveServer = strings.TrimSpace(flags.dnsServer)
			if err := validateHostname(req.Hostname); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addDNSMonitorFlags(cmd, &flags)
	return cmd
}

func newMonitorAddPushCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Add a passive push monitor",
		Run: func(cmd *cobra.Command, args []string) {
			req := common.toRequest()
			req.Type = models.MonitorTypePush
			if err := runMonitorAdd(rt, req, true); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	return cmd
}

func newMonitorAddDockerCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags
	var flags dockerMonitorFlags

	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Add a Docker container monitor",
		Run: func(cmd *cobra.Command, args []string) {
			req := common.toRequest()
			req.Type = models.MonitorTypeDocker
			req.DockerContainer = strings.TrimSpace(flags.container)
			req.DockerHost = flags.dockerHost
			if req.DockerContainer == "" {
				fmt.Fprintln(os.Stderr, "Error: --container is required")
				os.Exit(1)
			}
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addDockerMonitorFlags(cmd, &flags)
	return cmd
}

func newMonitorAddSMTPCmd(rt Runtime) *cobra.Command {
	var common commonMonitorFlags
	var flags smtpMonitorFlags

	cmd := &cobra.Command{
		Use:   "smtp",
		Short: "Add an SMTP server monitor",
		Run: func(cmd *cobra.Command, args []string) {
			security, err := validateSMTPSecurity(flags.smtpSecurity)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			req := common.toRequest()
			req.Type = models.MonitorTypeSMTP
			req.Hostname = strings.TrimSpace(flags.hostname)
			req.Port = flags.port
			req.SMTPSecurity = security
			req.SMTPUsername = flags.username
			req.SMTPPassword = flags.password
			req.SMTPFrom = flags.from
			req.SMTPTo = flags.to
			if err := validateHostname(req.Hostname); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := validatePort(req.Port); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addSMTPMonitorFlags(cmd, &flags)
	return cmd
}

func newMonitorAddPostgresCmd(rt Runtime) *cobra.Command {
	return newMonitorAddDatabaseCmd(rt, "postgres", "Add a PostgreSQL monitor", models.MonitorTypePostgres, 5432)
}

func newMonitorAddMySQLCmd(rt Runtime) *cobra.Command {
	return newMonitorAddDatabaseCmd(rt, "mysql", "Add a MySQL/MariaDB monitor", models.MonitorTypeMySQL, 3306)
}

func newMonitorAddSQLServerCmd(rt Runtime) *cobra.Command {
	return newMonitorAddDatabaseCmd(rt, "sqlserver", "Add a SQL Server monitor", models.MonitorTypeSQLServer, 1433)
}

func newMonitorAddDatabaseCmd(rt Runtime, use, short string, monitorType models.MonitorType, defaultPort int) *cobra.Command {
	var common commonMonitorFlags
	var flags databaseMonitorFlags

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Run: func(cmd *cobra.Command, args []string) {
			req := common.toRequest()
			req.Type = monitorType
			req.Hostname = strings.TrimSpace(flags.hostname)
			req.Port = flags.port
			req.DatabaseName = flags.dbName
			req.DatabaseUsername = flags.username
			req.DatabasePassword = flags.password
			req.DatabaseQuery = flags.query
			if err := validateHostname(req.Hostname); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := validatePort(req.Port); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := runMonitorAdd(rt, req, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	addCommonMonitorFlags(cmd, &common)
	addDatabaseMonitorFlags(cmd, &flags, defaultPort)
	return cmd
}

func buildHTTPAddRequest(monitorType models.MonitorType, common commonMonitorFlags, flags httpMonitorFlags) (models.AddMonitorRequest, error) {
	method, err := validateHTTPMethod(flags.method)
	if err != nil {
		return models.AddMonitorRequest{}, err
	}
	acceptedCodes, err := parseAcceptedCodes(flags.acceptedCodes)
	if err != nil {
		return models.AddMonitorRequest{}, err
	}
	if err := validateHeadersJSON(flags.headers); err != nil {
		return models.AddMonitorRequest{}, err
	}
	req := common.toRequest()
	req.Type = monitorType
	req.URL = strings.TrimSpace(flags.url)
	req.Method = method
	req.Body = flags.body
	req.Headers = flags.headers
	req.IgnoreTLS = flags.ignoreTLS
	req.BasicAuthUser = flags.basicAuthUser
	req.BasicAuthPass = flags.basicAuthPass
	req.AcceptedStatCodes = acceptedCodes
	req.MaxRedirects = flags.maxRedirects
	if strings.TrimSpace(req.URL) == "" {
		return models.AddMonitorRequest{}, fmt.Errorf("--url is required")
	}
	return req, nil
}

func runMonitorAdd(rt Runtime, req models.AddMonitorRequest, includePushDetails bool) error {
	api, ctx, cancel, err := connect(rt)
	if err != nil {
		return err
	}
	defer cancel()
	defer api.Disconnect()

	monitorID, err := api.AddMonitor(ctx, req)
	if err != nil {
		return err
	}

	if includePushDetails {
		monitor, err := api.GetMonitor(ctx, monitorID)
		if err != nil {
			return err
		}
		pushToken := monitor.PushToken
		if strings.TrimSpace(pushToken) == "" {
			pushToken = req.PushToken
		}
		pushURL := buildPushURL(rt.URL(), pushToken)
		if rt.outputFormat() == "json" {
			return printJSON(map[string]any{
				"id":        monitorID,
				"name":      req.Name,
				"type":      req.Type,
				"pushToken": pushToken,
				"pushURL":   pushURL,
			})
		}
		fmt.Println("Push monitor created!")
		fmt.Printf("  ID:        %d\n", monitorID)
		fmt.Printf("  Push URL:  %s\n\n", pushURL)
		fmt.Println("Use this URL to trigger a heartbeat from your CI/CD pipeline.")
		return nil
	}

	if rt.outputFormat() == "json" {
		return printJSON(map[string]any{"id": monitorID, "name": req.Name, "type": req.Type})
	}

	fmt.Printf("Monitor created with ID %d\n", monitorID)
	return nil
}

type editFlags struct {
	name          string
	url           string
	hostname      string
	port          int
	interval      int
	retryInterval int
	maxRetries    int
	timeout       int
	method        string
	keyword       string
	invertKeyword bool
	ignoreTLS     bool
	body          string
	headers       string
	basicAuthUser string
	basicAuthPass string
	acceptedCodes string
	paused        bool
}

func newMonitorEditCmd(rt Runtime) *cobra.Command {
	var flags editFlags

	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a monitor",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := runMonitorEdit(rt, cmd, args[0], flags); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&flags.name, "name", "", "Monitor name")
	cmd.Flags().StringVar(&flags.url, "url", "", "Target URL")
	cmd.Flags().StringVar(&flags.hostname, "hostname", "", "Hostname or IP")
	cmd.Flags().IntVar(&flags.port, "port", 0, "TCP port number")
	cmd.Flags().IntVar(&flags.interval, "interval", 0, "Check interval in seconds")
	cmd.Flags().IntVar(&flags.retryInterval, "retry-interval", 0, "Retry interval in seconds")
	cmd.Flags().IntVar(&flags.maxRetries, "max-retries", 0, "Max retries before DOWN")
	cmd.Flags().IntVar(&flags.timeout, "timeout", 0, "Timeout in seconds")
	cmd.Flags().StringVar(&flags.method, "method", "", "HTTP method GET|POST|PUT|PATCH|DELETE")
	cmd.Flags().StringVar(&flags.keyword, "keyword", "", "Keyword to search in response")
	cmd.Flags().BoolVar(&flags.invertKeyword, "invert-keyword", false, "Alert when keyword IS found")
	cmd.Flags().BoolVar(&flags.ignoreTLS, "ignore-tls", false, "Ignore TLS/SSL errors")
	cmd.Flags().StringVar(&flags.body, "body", "", "Request body")
	cmd.Flags().StringVar(&flags.headers, "headers", "", `JSON headers, e.g. '{"Authorization":"Bearer token"}'`)
	cmd.Flags().StringVar(&flags.basicAuthUser, "basic-auth-user", "", "Basic auth username")
	cmd.Flags().StringVar(&flags.basicAuthPass, "basic-auth-pass", "", "Basic auth password")
	cmd.Flags().StringVar(&flags.acceptedCodes, "accepted-codes", "", "Comma-separated, e.g. '200-299,404'")
	cmd.Flags().BoolVar(&flags.paused, "paused", false, "true = pause, false = resume")

	return cmd
}

func runMonitorEdit(rt Runtime, cmd *cobra.Command, rawID string, flags editFlags) error {
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

	req := monitorToAddMonitorRequest(monitor)
	req.Type = monitor.Type

	if cmd.Flags().Changed("name") {
		req.Name = flags.name
	}
	if cmd.Flags().Changed("url") {
		req.URL = strings.TrimSpace(flags.url)
	}
	if cmd.Flags().Changed("hostname") {
		req.Hostname = strings.TrimSpace(flags.hostname)
	}
	if cmd.Flags().Changed("port") {
		req.Port = flags.port
	}
	if cmd.Flags().Changed("interval") {
		req.Interval = flags.interval
	}
	if cmd.Flags().Changed("retry-interval") {
		req.RetryInterval = flags.retryInterval
	}
	if cmd.Flags().Changed("max-retries") {
		req.MaxRetries = flags.maxRetries
	}
	if cmd.Flags().Changed("timeout") {
		req.Timeout = flags.timeout
	}
	if cmd.Flags().Changed("method") {
		method, err := validateHTTPMethod(flags.method)
		if err != nil {
			return err
		}
		req.Method = method
	}
	if cmd.Flags().Changed("keyword") {
		req.Keyword = flags.keyword
	}
	if cmd.Flags().Changed("invert-keyword") {
		req.InvertKeyword = flags.invertKeyword
	}
	if cmd.Flags().Changed("ignore-tls") {
		req.IgnoreTLS = flags.ignoreTLS
	}
	if cmd.Flags().Changed("body") {
		req.Body = flags.body
	}
	if cmd.Flags().Changed("headers") {
		if err := validateHeadersJSON(flags.headers); err != nil {
			return err
		}
		req.Headers = flags.headers
	}
	if cmd.Flags().Changed("basic-auth-user") {
		req.BasicAuthUser = flags.basicAuthUser
	}
	if cmd.Flags().Changed("basic-auth-pass") {
		req.BasicAuthPass = flags.basicAuthPass
	}
	if cmd.Flags().Changed("accepted-codes") {
		acceptedCodes, err := parseAcceptedCodes(flags.acceptedCodes)
		if err != nil {
			return err
		}
		req.AcceptedStatCodes = acceptedCodes
	}
	if cmd.Flags().Changed("paused") {
		req.Active = !flags.paused
	}

	if err := validateMonitorRequest(req); err != nil {
		return err
	}

	if err := api.EditMonitor(ctx, models.EditMonitorRequest{ID: monitorID, AddMonitorRequest: req}); err != nil {
		return err
	}

	if rt.outputFormat() == "json" {
		return printJSON(map[string]any{"id": monitorID, "name": req.Name, "type": req.Type})
	}
	fmt.Printf("Monitor %d updated successfully.\n", monitorID)
	return nil
}

func monitorToAddMonitorRequest(m *models.Monitor) models.AddMonitorRequest {
	return models.AddMonitorRequest{
		Type:                     m.Type,
		Name:                     m.Name,
		URL:                      m.URL,
		Hostname:                 m.Hostname,
		Port:                     m.Port,
		Method:                   m.Method,
		Interval:                 m.Interval,
		RetryInterval:            m.RetryInterval,
		MaxRetries:               m.MaxRetries,
		Timeout:                  m.Timeout,
		Active:                   m.Active.IsActive(),
		Body:                     m.Body,
		Headers:                  m.Headers,
		BasicAuthUser:            m.BasicAuthUser,
		BasicAuthPass:            m.BasicAuthPass,
		IgnoreTLS:                m.IgnoreTLS,
		MaxRedirects:             m.MaxRedirects,
		Keyword:                  m.Keyword,
		InvertKeyword:            m.InvertKeyword,
		DNSResolveType:           m.DNSResolveType,
		DNSResolveServer:         m.DNSResolveServer,
		PushToken:                m.PushToken,
		DockerContainer:          m.DockerContainer,
		DockerHost:               m.DockerHost,
		SMTPSecurity:             m.SMTPSecurity,
		SMTPUsername:             m.SMTPUsername,
		SMTPPassword:             m.SMTPPassword,
		SMTPFrom:                 m.SMTPFrom,
		SMTPTo:                   m.SMTPTo,
		DatabaseName:             m.DatabaseName,
		DatabaseUsername:         m.DatabaseUsername,
		DatabasePassword:         m.DatabasePassword,
		DatabaseQuery:            m.DatabaseQuery,
		DatabaseConnectionString: m.DatabaseConnectionString,
		ParentID:                 m.ParentID,
		ProxyID:                  m.ProxyID,
		NotificationIDList:       cloneNotificationMap(m.NotificationIDList),
		AcceptedStatCodes:        cloneStrings(m.AcceptedStatCodes),
		Conditions:               cloneConditions(m.Conditions),
	}
}

func newMonitorTypesCmd(rt Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List available monitor types",
		Run: func(cmd *cobra.Command, args []string) {
			types := []struct {
				Type        string `json:"type"`
				Description string `json:"description"`
			}{
				{Type: "http", Description: "HTTP/HTTPS endpoint monitoring"},
				{Type: "keyword", Description: "HTTP response keyword check"},
				{Type: "ping", Description: "ICMP ping"},
				{Type: "port", Description: "TCP port check"},
				{Type: "dns", Description: "DNS record resolution"},
				{Type: "push", Description: "Passive push (use in CI/CD)"},
				{Type: "docker", Description: "Docker container status"},
				{Type: "smtp", Description: "SMTP email server"},
				{Type: "postgres", Description: "PostgreSQL query"},
				{Type: "mysql", Description: "MySQL/MariaDB query"},
				{Type: "sqlserver", Description: "SQL Server query"},
			}
			if rt.outputFormat() == "json" {
				if err := printJSON(types); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}
				return
			}
			w := newTableWriter()
			fmt.Fprintln(w, styleHeader.Render("TYPE\tDESCRIPTION"))
			for _, item := range types {
				fmt.Fprintf(w, "%s\t%s\n", item.Type, item.Description)
			}
			if err := w.Flush(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
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

func validateHTTPMethod(method string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(method))
	switch value {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return value, nil
	default:
		return "", fmt.Errorf("invalid --method %q (expected GET|POST|PUT|PATCH|DELETE)", method)
	}
}

func validateDNSResolveType(resolveType string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(resolveType))
	switch value {
	case "A", "AAAA", "CAA", "CNAME", "MX", "NS", "PTR", "SOA", "SRV", "TXT":
		return value, nil
	default:
		return "", fmt.Errorf("invalid --resolve-type %q", resolveType)
	}
}

func validateSMTPSecurity(security string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(security))
	switch value {
	case "none", "starttls", "tls":
		return value, nil
	default:
		return "", fmt.Errorf("invalid --smtp-security %q (expected none|starttls|tls)", security)
	}
}

func parseAcceptedCodes(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("--accepted-codes must contain at least one value")
	}
	return values, nil
}

func validateHeadersJSON(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return fmt.Errorf("invalid --headers JSON: %w", err)
	}
	return nil
}

func validateHostname(hostname string) error {
	if strings.TrimSpace(hostname) == "" {
		return fmt.Errorf("--hostname is required")
	}
	return nil
}

func validatePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid --port %d", port)
	}
	return nil
}

func validateMonitorRequest(req models.AddMonitorRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("monitor name cannot be empty")
	}
	if req.Interval < 0 || req.RetryInterval < 0 || req.MaxRetries < 0 || req.Timeout < 0 {
		return fmt.Errorf("interval, retry-interval, max-retries, and timeout must be non-negative")
	}
	switch req.Type {
	case models.MonitorTypeHTTP, models.MonitorTypeKeyword:
		if strings.TrimSpace(req.URL) == "" {
			return fmt.Errorf("--url is required for %s monitors", req.Type)
		}
		if _, err := validateHTTPMethod(req.Method); err != nil {
			return err
		}
		if err := validateHeadersJSON(req.Headers); err != nil {
			return err
		}
		if len(req.AcceptedStatCodes) == 0 {
			return fmt.Errorf("accepted status codes cannot be empty")
		}
		if req.Type == models.MonitorTypeKeyword && strings.TrimSpace(req.Keyword) == "" {
			return fmt.Errorf("--keyword is required for keyword monitors")
		}
	case models.MonitorTypePing:
		return validateHostname(req.Hostname)
	case models.MonitorTypePort:
		if err := validateHostname(req.Hostname); err != nil {
			return err
		}
		return validatePort(req.Port)
	case models.MonitorTypeDNS:
		if err := validateHostname(req.Hostname); err != nil {
			return err
		}
		_, err := validateDNSResolveType(req.DNSResolveType)
		return err
	case models.MonitorTypeDocker:
		if strings.TrimSpace(req.DockerContainer) == "" {
			return fmt.Errorf("--container is required for docker monitors")
		}
	case models.MonitorTypeSMTP:
		if err := validateHostname(req.Hostname); err != nil {
			return err
		}
		if err := validatePort(req.Port); err != nil {
			return err
		}
		_, err := validateSMTPSecurity(req.SMTPSecurity)
		return err
	case models.MonitorTypePostgres, models.MonitorTypeMySQL, models.MonitorTypeSQLServer:
		if err := validateHostname(req.Hostname); err != nil {
			return err
		}
		return validatePort(req.Port)
	case models.MonitorTypePush:
		return nil
	default:
		return nil
	}
	return nil
}

func cloneNotificationMap(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return map[string]bool{}
	}
	dst := make(map[string]bool, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneConditions(src []models.MonitorCondition) []models.MonitorCondition {
	if len(src) == 0 {
		return []models.MonitorCondition{}
	}
	dst := make([]models.MonitorCondition, len(src))
	copy(dst, src)
	return dst
}

func buildPushURL(baseURL, pushToken string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/push/" + strings.TrimSpace(pushToken)
}
