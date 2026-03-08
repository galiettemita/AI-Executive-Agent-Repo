package sandbox

import "testing"

func TestMCPDefaultProfiles(t *testing.T) {
	t.Parallel()

	defaults := DefaultProfiles()
	if len(defaults) != 3 {
		t.Fatalf("expected 3 default profiles, got %d", len(defaults))
	}

	strict := defaults["strict"]
	if strict.AllowNetwork {
		t.Fatalf("strict profile should not allow network")
	}
	if strict.AllowFilesystemWrite {
		t.Fatalf("strict profile should not allow filesystem write")
	}
	if strict.MaxMemoryMB != 256 {
		t.Fatalf("strict profile expected 256MB, got %d", strict.MaxMemoryMB)
	}
	if strict.MaxCPUSeconds != 30 {
		t.Fatalf("strict profile expected 30s CPU, got %d", strict.MaxCPUSeconds)
	}

	standard := defaults["standard"]
	if !standard.AllowNetwork {
		t.Fatalf("standard profile should allow network")
	}
	if standard.MaxMemoryMB != 512 {
		t.Fatalf("standard profile expected 512MB, got %d", standard.MaxMemoryMB)
	}

	permissive := defaults["permissive"]
	if permissive.MaxMemoryMB != 1024 {
		t.Fatalf("permissive profile expected 1024MB, got %d", permissive.MaxMemoryMB)
	}
	if permissive.MaxCPUSeconds != 120 {
		t.Fatalf("permissive profile expected 120s CPU, got %d", permissive.MaxCPUSeconds)
	}
}

func TestMCPRegisterProfile(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()

	err := svc.RegisterProfile(MCPSandboxProfile{})
	if err == nil {
		t.Fatalf("expected error for empty server_id")
	}

	err = svc.RegisterProfile(MCPSandboxProfile{ServerID: "srv-1", Name: "invalid"})
	if err == nil {
		t.Fatalf("expected error for invalid profile name")
	}

	err = svc.RegisterProfile(MCPSandboxProfile{
		ServerID:     "my-server",
		Name:         "custom",
		AllowedPaths: []string{"/data"},
		DeniedPaths:  []string{"/secret"},
		AllowNetwork: true,
		MaxMemoryMB:  128,
		MaxCPUSeconds: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p, err := svc.GetProfile("my-server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "custom" || p.MaxMemoryMB != 128 {
		t.Fatalf("unexpected profile: %+v", p)
	}
}

func TestMCPGetProfileNotFound(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()
	_, err := svc.GetProfile("nonexistent-server")
	if err == nil {
		t.Fatalf("expected error for missing profile")
	}
}

func TestMCPValidateToolExecutionAllowed(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()
	_ = svc.RegisterProfile(MCPSandboxProfile{
		ServerID:            "srv-1",
		Name:                "standard",
		AllowedPaths:        []string{"/tmp", "/workspace"},
		AllowNetwork:        true,
		AllowFilesystemWrite: true,
		MaxMemoryMB:         512,
		MaxCPUSeconds:       60,
	})

	decision, err := svc.ValidateToolExecution("srv-1", "read_file", map[string]any{
		"path": "/tmp/data.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed, got reason: %s", decision.Reason)
	}
	if decision.AppliedProfile != "standard" {
		t.Fatalf("expected standard profile, got %s", decision.AppliedProfile)
	}
}

func TestMCPValidateToolExecutionDeniedPath(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()
	_ = svc.RegisterProfile(MCPSandboxProfile{
		ServerID:     "srv-2",
		Name:         "strict",
		AllowedPaths: []string{"/tmp"},
		MaxMemoryMB:  256,
		MaxCPUSeconds: 30,
	})

	decision, err := svc.ValidateToolExecution("srv-2", "read_file", map[string]any{
		"path": "/etc/shadow",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected denied for path /etc/shadow")
	}
}

func TestMCPValidateToolExecutionNoProfile(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()
	decision, err := svc.ValidateToolExecution("unknown-server", "read_file", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected denied for unknown server")
	}
}

func TestMCPValidateToolExecutionNetworkDenied(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()
	_ = svc.RegisterProfile(MCPSandboxProfile{
		ServerID:     "srv-nonet",
		Name:         "strict",
		AllowNetwork: false,
		MaxMemoryMB:  256,
		MaxCPUSeconds: 30,
	})

	decision, _ := svc.ValidateToolExecution("srv-nonet", "http_fetch", nil)
	if decision.Allowed {
		t.Fatalf("expected network tool denied under strict profile")
	}
}

func TestMCPValidateToolExecutionWriteDenied(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()
	_ = svc.RegisterProfile(MCPSandboxProfile{
		ServerID:            "srv-nofs",
		Name:                "strict",
		AllowFilesystemWrite: false,
		MaxMemoryMB:         256,
		MaxCPUSeconds:       30,
	})

	decision, _ := svc.ValidateToolExecution("srv-nofs", "write_file", map[string]any{})
	if decision.Allowed {
		t.Fatalf("expected filesystem write denied under strict profile")
	}
}

func TestMCPValidateCommand(t *testing.T) {
	t.Parallel()

	profile := &MCPSandboxProfile{
		DeniedCommands: []string{"rm", "shutdown", "reboot"},
	}

	if allowed, _ := ValidateCommand(profile, "ls -la"); !allowed {
		t.Fatalf("ls should be allowed")
	}
	if allowed, _ := ValidateCommand(profile, "rm -rf /tmp/data"); allowed {
		t.Fatalf("rm should be denied")
	}
	if allowed, _ := ValidateCommand(profile, "shutdown -h now"); allowed {
		t.Fatalf("shutdown should be denied")
	}
}

func TestMCPValidateCommandInArgs(t *testing.T) {
	t.Parallel()

	svc := NewMCPSandboxService()
	_ = svc.RegisterProfile(MCPSandboxProfile{
		ServerID:       "srv-cmd",
		Name:           "strict",
		DeniedCommands: []string{"rm", "kill"},
		MaxMemoryMB:    256,
		MaxCPUSeconds:  30,
	})

	decision, _ := svc.ValidateToolExecution("srv-cmd", "exec", map[string]any{
		"command": "rm -rf /tmp",
	})
	if decision.Allowed {
		t.Fatalf("expected denied for rm command in args")
	}
}

func TestMCPValidateFilePath(t *testing.T) {
	t.Parallel()

	profile := &MCPSandboxProfile{
		AllowedPaths: []string{"/tmp", "/workspace"},
		DeniedPaths:  []string{"/etc/shadow"},
	}

	if allowed, _ := ValidateFilePath(profile, "/tmp/file.txt"); !allowed {
		t.Fatalf("/tmp/file.txt should be allowed")
	}
	if allowed, _ := ValidateFilePath(profile, "/workspace/project/main.go"); !allowed {
		t.Fatalf("/workspace subpath should be allowed")
	}
	if allowed, _ := ValidateFilePath(profile, "/etc/shadow"); allowed {
		t.Fatalf("/etc/shadow should be denied")
	}
	if allowed, _ := ValidateFilePath(profile, "/home/user/data"); allowed {
		t.Fatalf("/home should not be in allowed paths")
	}
}
