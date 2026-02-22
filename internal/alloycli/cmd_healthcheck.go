package alloycli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func healthcheckCommand() *cobra.Command {
	h := &alloyHealthcheck{
		addr:    "127.0.0.1:12345",
		path:    "/-/ready",
		timeout: 5 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "healthcheck [flags]",
		Short: "Check the health of a running Alloy instance",
		Long: `The healthcheck subcommand queries a running Alloy instance to determine
its health.

By default, healthcheck queries the /-/ready endpoint of the instance running
at 127.0.0.1:12345. A successful health check returns exit code 0. A failed
health check returns a non-zero exit code.

Use healthcheck in Docker HEALTHCHECK instructions or other orchestration
tools to monitor the health of Alloy without external utilities like curl
or wget.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,

		RunE: func(_ *cobra.Command, _ []string) error {
			return h.Run()
		},
	}

	cmd.Flags().StringVar(&h.url, "url", h.url, "Full URL to check. Overrides --addr and --path.")
	cmd.Flags().StringVar(&h.addr, "addr", h.addr, "Address of the running Alloy instance.")
	cmd.Flags().StringVar(&h.path, "path", h.path, "Path to the health endpoint.")
	cmd.Flags().DurationVar(&h.timeout, "timeout", h.timeout, "Timeout for the HTTP request.")

	return cmd
}

type alloyHealthcheck struct {
	url     string
	addr    string
	path    string
	timeout time.Duration
}

func (h *alloyHealthcheck) Run() error {
	targetURL := h.url
	if targetURL == "" {
		targetURL = fmt.Sprintf("http://%s%s", h.addr, h.path)
	} else if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}

	client := &http.Client{}

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		fmt.Fprint(os.Stdout, string(body))
		return nil
	}

	fmt.Fprint(os.Stderr, string(body))
	return fmt.Errorf("unhealthy: HTTP status %d", resp.StatusCode)
}
