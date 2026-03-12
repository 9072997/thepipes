//go:build windows

package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"

	"github.com/9072997/thepipes/internal/eventlog"
	"github.com/9072997/thepipes/internal/installcfg"
	"github.com/9072997/thepipes/internal/logger"
)

type serviceHandler struct {
	manifest   AppManifest
	installDir string
	dataDir    string
}

// serviceMode runs the binary as a Windows service via the SCM.
func serviceMode(manifest AppManifest) error {
	h := &serviceHandler{
		manifest:   manifest,
		installDir: getInstallDir(manifest.Name),
		dataDir:    getDataDir(manifest.Name),
	}
	return svc.Run(slug(manifest.Name), h)
}

// Execute is called by the SCM when the service starts.
func (h *serviceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	// Rotate log at service start.
	_ = logger.Rotate(h.dataDir)

	lw, err := logger.Open(h.dataDir)
	if err != nil {
		changes <- svc.Status{State: svc.Stopped}
		return false, 1
	}
	defer lw.Close()

	log := func(msg string) {
		fmt.Fprintf(lw, "[%s] %s\n", time.Now().Format(time.RFC3339), msg)
	}

	log("Service starting")

	// Read install config.
	cfg, err := installcfg.Read(h.dataDir)
	if err != nil {
		log(fmt.Sprintf("read config: %v", err))
		changes <- svc.Status{State: svc.Stopped}
		return false, 2
	}

	vars := MergeVars(BuiltinVars(h.manifest), cfg)

	// Expand the command.
	expanded := Expand(h.manifest.Command, vars)
	parts := splitCommand(expanded)
	if len(parts) == 0 {
		log("empty command")
		changes <- svc.Status{State: svc.Stopped}
		return false, 3
	}

	exe := parts[0]
	if !filepath.IsAbs(exe) {
		exe = filepath.Join(h.installDir, exe)
	}

	// Open Event Log source (best-effort).
	evtLog, _ := eventlog.Open(h.manifest.Name)

	// Backoff delays in seconds: 1, 2, 4, 8, 16, 32, then cap at 5 min.
	backoffDelays := []time.Duration{1, 2, 4, 8, 16, 32, 300}
	backoffIdx := 0

	var cmd *exec.Cmd
	childDone := make(chan error, 1)

	startChild := func() {
		cmd = exec.Command(exe, parts[1:]...)
		cmd.Dir = h.dataDir
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}

		// Inject config as environment variables.
		env := os.Environ()
		for k, v := range vars {
			env = append(env, strings.ToUpper(k)+"="+v)
		}
		cmd.Env = env

		// Pipe stdout and stderr for log parsing.
		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			log(fmt.Sprintf("start child: %v", err))
			childDone <- err
			return
		}

		log(fmt.Sprintf("Child process started (pid %d)", cmd.Process.Pid))

		scanLines := func(r io.Reader) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				line := scanner.Text()
				sev, msg := eventlog.ParsePrefix(line)
				fmt.Fprintln(lw, msg)
				if sev != "" && evtLog != nil {
					_ = eventlog.Write(evtLog, sev, line[len(sev)+1:])
				}
			}
		}
		go scanLines(stdoutPipe)
		go scanLines(stderrPipe)

		go func() { childDone <- cmd.Wait() }()
	}

	startChild()
	startTime := time.Now()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	// Start update check goroutine if configured.
	if cfg["auto_updates"] != "false" && h.manifest.UpdateURL != "" {
		go checkForUpdateLoop(h.manifest, h.installDir, lw)
	}

	for {
		select {
		case req := <-r:
			switch req.Cmd {
			case svc.Stop, svc.Shutdown:
				log("Service stopping")
				changes <- svc.Status{State: svc.StopPending}
				if cmd != nil && cmd.Process != nil {
					// Send CTRL_BREAK_EVENT to the process group.
					dll := syscall.NewLazyDLL("kernel32.dll")
					p := dll.NewProc("GenerateConsoleCtrlEvent")
					p.Call(syscall.CTRL_BREAK_EVENT, uintptr(cmd.Process.Pid))

					// Wait up to 10s, then kill.
					timer := time.NewTimer(10 * time.Second)
					select {
					case <-childDone:
					case <-timer.C:
						_ = cmd.Process.Kill()
						<-childDone
					}
					timer.Stop()
				}
				changes <- svc.Status{State: svc.Stopped}
				return false, 0
			case svc.Interrogate:
				changes <- req.CurrentStatus
			}

		case childErr := <-childDone:
			log(fmt.Sprintf("Child process exited: %v", childErr))

			// Reset backoff if process ran for more than a minute.
			if time.Since(startTime) > time.Minute {
				backoffIdx = 0
			}

			delay := backoffDelays[backoffIdx] * time.Second
			if backoffIdx < len(backoffDelays)-1 {
				backoffIdx++
			}
			log(fmt.Sprintf("Restarting in %v", delay))
			time.Sleep(delay)

			childDone = make(chan error, 1)
			startTime = time.Now()
			startChild()
		}
	}
}

// splitCommand splits a command string into tokens respecting quoted segments.
func splitCommand(cmd string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for _, r := range cmd {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' && !inQuote:
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}
