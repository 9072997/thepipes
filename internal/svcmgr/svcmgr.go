//go:build windows

package svcmgr

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Install registers the service with the Windows SCM.
// exe is the full path to the service binary.
// serviceAccount may be "" or "LocalService" for LocalService, or "DOMAIN\user" for a domain account.
func Install(svcName, displayName, description, exe, serviceAccount, password string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	// Remove existing service if present.
	existing, err := m.OpenService(svcName)
	if err == nil {
		existing.Delete()
		existing.Close()
		time.Sleep(500 * time.Millisecond)
	}

	cfg := mgr.Config{
		DisplayName: displayName,
		Description: description,
		StartType:   mgr.StartAutomatic,
	}

	switch serviceAccount {
	case "", "LocalService":
		cfg.ServiceStartName = "NT AUTHORITY\\LocalService"
	default:
		cfg.ServiceStartName = serviceAccount
		cfg.Password = password
	}

	s, err := m.CreateService(svcName, exe, cfg, "--service")
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	// Recovery actions: restart after 5s twice, then no action.
	_ = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.NoAction},
	}, 0)

	return nil
}

// Start starts the named service.
func Start(svcName string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(svcName)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close()

	return s.Start()
}

// Stop sends a stop signal and waits up to 30s for the service to stop.
func Stop(svcName string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(svcName)
	if err != nil {
		// Service not found - treat as already stopped.
		return nil
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		return nil
	}

	if _, err := s.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		status, err = s.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Stopped {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for service %q to stop", svcName)
}

// Delete stops (if running) and removes the named service from the SCM.
func Delete(svcName string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(svcName)
	if err != nil {
		return nil // already gone
	}
	defer s.Close()

	// Best-effort stop.
	_, _ = s.Control(svc.Stop)
	time.Sleep(2 * time.Second)

	return s.Delete()
}

// State returns the current service state. Returns svc.Stopped if the service does not exist.
func State(svcName string) (svc.State, error) {
	m, err := mgr.Connect()
	if err != nil {
		return svc.Stopped, fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(svcName)
	if err != nil {
		return svc.Stopped, nil
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return svc.Stopped, err
	}
	return status.State, nil
}
