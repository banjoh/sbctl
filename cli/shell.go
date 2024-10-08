package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/api"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

func ShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "shell",
		Short:         "Start interractive shell",
		Long:          `Start interractive shell`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			viper.SetEnvPrefix("sbctl")
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var kubeConfig string
			var bundleDir string
			deleteBundleDir := false

			logOutput := os.Stderr
			logFile, err := os.CreateTemp("", "sbctl-server-*.log")
			if err == nil {
				defer logFile.Close()
				defer os.RemoveAll(logFile.Name())
				fmt.Printf("SBCTL logs will be written to %s\n", logFile.Name())
				log.SetOutput(logFile)
				logOutput = logFile
			}

			cleanup := func() {
				if kubeConfig != "" {
					_ = os.RemoveAll(kubeConfig)
				}
				if deleteBundleDir && bundleDir != "" {
					_ = os.RemoveAll(bundleDir)
				}
			}

			go func() {
				signalChan := make(chan os.Signal, 1)
				// Handle Ctl-D to exit from shell
				signal.Notify(signalChan, os.Interrupt)
				<-signalChan
				cleanup()
				os.Exit(0)
			}()

			defer func() {
				// exit from shell using "exit" command
				cleanup()
			}()

			v := viper.GetViper()

			// This only works with generated config, so let's make sure we don't mess up user's real files.
			bundleLocation := v.GetString("support-bundle-location")
			if len(args) > 0 && args[0] != "" {
				bundleLocation = args[0]
			}
			if bundleLocation == "" {
				return errors.New("support-bundle-location is required")
			}

			if strings.HasPrefix(bundleLocation, "http") {
				token := v.GetString("token")
				if token == "" {
					return errors.New("token is required when downloading bundle")
				}

				fmt.Printf("Downloading bundle\n")

				dir, err := downloadAndExtractBundle(bundleLocation, token)
				if err != nil {
					return errors.Wrap(err, "failed to stat input path")
				}
				fmt.Printf("Bundle extracted to %s\n", dir)
				bundleDir = dir
				deleteBundleDir = true
			} else {
				fileInfo, err := os.Stat(bundleLocation)
				if err != nil {
					return errors.Wrap(err, "failed to stat input path")
				}

				bundleDir = bundleLocation
				if !fileInfo.IsDir() {
					deleteBundleDir = true
					bundleDir, err = os.MkdirTemp("", "sbctl-")
					if err != nil {
						return errors.Wrap(err, "failed to create temp dir")
					}

					err = sbctl.ExtractBundle(bundleLocation, bundleDir)
					if err != nil {
						return errors.Wrap(err, "failed to extract bundle")
					}
				}
			}

			clusterData, err := sbctl.FindClusterData(bundleDir)
			if err != nil {
				return errors.Wrap(err, "failed to find cluster data")
			}

			// If we did not find cluster data, just don't start the API server
			if clusterData.ClusterResourcesDir == "" {
				fmt.Println("No cluster resources found in bundle")
				fmt.Println("Starting new shell in downloaded bundle. Press Ctl-D when done to exit from the shell")
				return startShellAndWait(fmt.Sprintf("cd %s", bundleDir))
			}

			kubeConfig, err = api.StartAPIServer(clusterData, logOutput)
			if err != nil {
				return errors.Wrap(err, "failed to create api server")
			}
			defer os.RemoveAll(kubeConfig)

			cmds := []string{
				fmt.Sprintf("export KUBECONFIG=%s", kubeConfig),
			}
			if v.GetBool("cd-bundle") {
				cmds = append(cmds, fmt.Sprintf("cd %s", bundleDir))
			}
			fmt.Printf("Starting new shell with KUBECONFIG. Press Ctl-D when done to exit from the shell and stop sbctl server\n")
			return startShellAndWait(cmds...)
		},
	}

	cmd.Flags().StringP("support-bundle-location", "s", "", "path to support bundle archive, directory, or URL")
	cmd.Flags().StringP("token", "t", "", "API token for authentication when fetching on-line bundles")
	cmd.Flags().Bool("cd-bundle", false, "Change directory to the support bundle path after starting the shell")
	cmd.Flags().Bool("debug", false, "enable debug logging. This will include HTTP response bodies in logs.")
	return cmd
}

func startShellAndWait(cmds ...string) error {
	shellCmd := os.Getenv("SHELL")
	if shellCmd == "" {
		return errors.New("SHELL environment is required for shell command")
	}

	shellExec := exec.Command(shellCmd)
	shellExec.Env = os.Environ()
	shellPty, err := pty.Start(shellExec)
	if err != nil {
		return errors.Wrap(err, "failed to start shell")
	}

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, shellPty); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.
	defer func() { signal.Stop(ch); close(ch) }()

	// Set stdin to raw mode.
	oldState, err := term.MakeRaw(syscall.Stdin)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = term.Restore(syscall.Stdin, oldState)
		fmt.Printf("sbctl shell exited\n")
	}()

	// Setup the shell
	setupCmd := strings.Join(cmds, "\n") + "\n"
	_, _ = io.WriteString(shellPty, setupCmd)
	_, _ = io.CopyN(io.Discard, shellPty, 2*int64(len(setupCmd))) // Don't print to screen, terminal will echo anyway

	// Copy stdin to the pty and the pty to stdout.
	go func() { _, _ = io.Copy(shellPty, os.Stdin) }()
	go func() { _, _ = io.Copy(os.Stdout, shellPty) }()

	return shellExec.Wait()
}
