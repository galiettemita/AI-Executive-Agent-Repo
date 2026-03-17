package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// MCPSandboxProfile defines resource and access constraints for an MCP server.
type MCPSandboxProfile struct {
	ServerID             string   `json:"server_id"`
	Name                 string   `json:"name"` // strict | standard | permissive | custom
	AllowedPaths         []string `json:"allowed_paths"`
	DeniedPaths          []string `json:"denied_paths"`
	AllowNetwork         bool     `json:"allow_network"`
	AllowFilesystemWrite bool     `json:"allow_filesystem_write"`
	MaxMemoryMB          int      `json:"max_memory_mb"`
	MaxCPUSeconds        int      `json:"max_cpu_seconds"`
	AllowedEnvVars       []string `json:"allowed_env_vars"`
	DeniedCommands       []string `json:"denied_commands"`
	AllowedHosts         []string `json:"allowed_hosts,omitempty"`
}

// AllowsHost checks whether the given URL is permitted by this sandbox profile.
func (p *MCPSandboxProfile) AllowsHost(rawURL string) bool {
	if len(p.AllowedHosts) == 0 {
		return true
	}
	var host string
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		remainder := rawURL[idx+3:]
		if slashIdx := strings.Index(remainder, "/"); slashIdx >= 0 {
			host = remainder[:slashIdx]
		} else {
			host = remainder
		}
	}
	for _, allowed := range p.AllowedHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// SandboxDecision is the outcome of a tool-execution validation.
type SandboxDecision struct {
	Allowed        bool           `json:"allowed"`
	Reason         string         `json:"reason"`
	AppliedProfile string         `json:"applied_profile"`
	Constraints    map[string]any `json:"constraints"`
}

// MCPSandboxService manages per-server sandbox profiles and validates tool
// execution requests against them.
type MCPSandboxService struct {
	mu       sync.RWMutex
	profiles map[string]MCPSandboxProfile // keyed by ServerID
}

// NewMCPSandboxService returns a service pre-loaded with the three default
// profiles (strict / standard / permissive).
func NewMCPSandboxService() *MCPSandboxService {
	svc := &MCPSandboxService{
		profiles: make(map[string]MCPSandboxProfile),
	}
	for id, p := range DefaultProfiles() {
		p.ServerID = id
		svc.profiles[id] = p
	}
	return svc
}

// RegisterProfile stores (or overwrites) a sandbox profile for the given
// server. ServerID and Name are required.
func (s *MCPSandboxService) RegisterProfile(profile MCPSandboxProfile) error {
	if strings.TrimSpace(profile.ServerID) == "" {
		return fmt.Errorf("server_id is required")
	}
	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	validNames := map[string]bool{"strict": true, "standard": true, "permissive": true, "custom": true}
	if !validNames[profile.Name] {
		return fmt.Errorf("profile name must be one of strict, standard, permissive, custom; got %q", profile.Name)
	}
	if profile.MaxMemoryMB <= 0 {
		profile.MaxMemoryMB = 256
	}
	if profile.MaxCPUSeconds <= 0 {
		profile.MaxCPUSeconds = 30
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[profile.ServerID] = profile
	return nil
}

// GetProfile returns the profile for a server, or an error if none exists.
func (s *MCPSandboxService) GetProfile(serverID string) (*MCPSandboxProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.profiles[serverID]
	if !ok {
		return nil, fmt.Errorf("no sandbox profile registered for server %q", serverID)
	}
	return &p, nil
}

// ValidateToolExecution checks whether a tool invocation on the given server
// is allowed under its sandbox profile.
func (s *MCPSandboxService) ValidateToolExecution(serverID string, toolName string, args map[string]any) (*SandboxDecision, error) {
	profile, err := s.GetProfile(serverID)
	if err != nil {
		return &SandboxDecision{
			Allowed:        false,
			Reason:         fmt.Sprintf("no profile for server %q", serverID),
			AppliedProfile: "",
			Constraints:    nil,
		}, nil
	}

	decision := &SandboxDecision{
		Allowed:        true,
		Reason:         "ok",
		AppliedProfile: profile.Name,
		Constraints: map[string]any{
			"max_memory_mb":  profile.MaxMemoryMB,
			"max_cpu_seconds": profile.MaxCPUSeconds,
			"allow_network":  profile.AllowNetwork,
		},
	}

	// Check file-path arguments.
	for key, val := range args {
		if isPathArg(key) {
			if path, ok := val.(string); ok {
				if allowed, reason := ValidateFilePath(profile, path); !allowed {
					decision.Allowed = false
					decision.Reason = reason
					return decision, nil
				}
			}
		}
	}

	// Check command arguments.
	if cmd, ok := args["command"].(string); ok {
		if allowed, reason := ValidateCommand(profile, cmd); !allowed {
			decision.Allowed = false
			decision.Reason = reason
			return decision, nil
		}
	}

	// Network check.
	if isNetworkTool(toolName) && !profile.AllowNetwork {
		decision.Allowed = false
		decision.Reason = "network access denied by profile"
		return decision, nil
	}

	// Filesystem write check.
	if isWriteTool(toolName) && !profile.AllowFilesystemWrite {
		decision.Allowed = false
		decision.Reason = "filesystem write denied by profile"
		return decision, nil
	}

	return decision, nil
}

// ValidateFilePath checks a path against the profile's allowed/denied lists.
func ValidateFilePath(profile *MCPSandboxProfile, path string) (bool, string) {
	cleanPath := filepath.Clean(path)

	// Deny list takes priority.
	for _, pattern := range profile.DeniedPaths {
		matched, err := filepath.Match(pattern, cleanPath)
		if err == nil && matched {
			return false, fmt.Sprintf("path %q matches denied pattern %q", path, pattern)
		}
		// Also check prefix-based denial for directory patterns.
		if strings.HasSuffix(pattern, "/*") {
			dir := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(cleanPath, dir+"/") || cleanPath == dir {
				return false, fmt.Sprintf("path %q is under denied directory %q", path, dir)
			}
		}
		if strings.HasPrefix(cleanPath, filepath.Clean(pattern)+"/") {
			return false, fmt.Sprintf("path %q is under denied directory %q", path, pattern)
		}
	}

	// If there is an allow list, the path must match at least one entry.
	if len(profile.AllowedPaths) > 0 {
		for _, pattern := range profile.AllowedPaths {
			matched, err := filepath.Match(pattern, cleanPath)
			if err == nil && matched {
				return true, "ok"
			}
			// Prefix-based allow for directory patterns.
			cleanPattern := filepath.Clean(pattern)
			if strings.HasPrefix(cleanPath, cleanPattern+"/") || cleanPath == cleanPattern {
				return true, "ok"
			}
		}
		return false, fmt.Sprintf("path %q not in allowed paths", path)
	}

	return true, "ok"
}

// ValidateCommand checks a command string against the profile's denied
// commands list.
func ValidateCommand(profile *MCPSandboxProfile, cmd string) (bool, string) {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	for _, denied := range profile.DeniedCommands {
		deniedLower := strings.ToLower(strings.TrimSpace(denied))
		if deniedLower == "" {
			continue
		}
		// Match if the command starts with the denied command or contains it
		// as a standalone token.
		if lower == deniedLower || strings.HasPrefix(lower, deniedLower+" ") || strings.Contains(lower, " "+deniedLower+" ") || strings.HasSuffix(lower, " "+deniedLower) {
			return false, fmt.Sprintf("command %q is denied by profile", denied)
		}
	}
	return true, "ok"
}

// DefaultProfiles returns the three built-in profile templates.
func DefaultProfiles() map[string]MCPSandboxProfile {
	return map[string]MCPSandboxProfile{
		"strict": {
			ServerID:            "strict",
			Name:                "strict",
			AllowedPaths:        []string{"/tmp"},
			DeniedPaths:         []string{"/etc", "/var", "/root", "/home"},
			AllowNetwork:        false,
			AllowFilesystemWrite: false,
			MaxMemoryMB:         256,
			MaxCPUSeconds:       30,
			AllowedEnvVars:      []string{"PATH", "HOME"},
			DeniedCommands:      []string{"rm", "chmod", "chown", "kill", "shutdown", "reboot", "dd", "mkfs"},
		},
		"standard": {
			ServerID:            "standard",
			Name:                "standard",
			AllowedPaths:        []string{"/tmp", "/workspace"},
			DeniedPaths:         []string{"/etc/shadow", "/etc/passwd", "/root"},
			AllowNetwork:        true,
			AllowFilesystemWrite: true,
			MaxMemoryMB:         512,
			MaxCPUSeconds:       60,
			AllowedEnvVars:      []string{"PATH", "HOME", "USER", "LANG"},
			DeniedCommands:      []string{"rm -rf /", "shutdown", "reboot", "dd", "mkfs"},
		},
		"permissive": {
			ServerID:            "permissive",
			Name:                "permissive",
			AllowedPaths:        []string{"/tmp", "/workspace", "/home", "/var/data"},
			DeniedPaths:         []string{"/etc/shadow"},
			AllowNetwork:        true,
			AllowFilesystemWrite: true,
			MaxMemoryMB:         1024,
			MaxCPUSeconds:       120,
			AllowedEnvVars:      []string{},
			DeniedCommands:      []string{"shutdown", "reboot", "mkfs"},
		},
	}
}

// isPathArg heuristically identifies argument keys that carry file paths.
func isPathArg(key string) bool {
	lower := strings.ToLower(key)
	return lower == "path" || lower == "file" || lower == "filepath" ||
		lower == "filename" || lower == "directory" || lower == "dir" ||
		strings.HasSuffix(lower, "_path") || strings.HasSuffix(lower, "_file") ||
		strings.HasSuffix(lower, "_dir")
}

func isNetworkTool(toolName string) bool {
	lower := strings.ToLower(toolName)
	return strings.Contains(lower, "http") || strings.Contains(lower, "fetch") ||
		strings.Contains(lower, "request") || strings.Contains(lower, "curl") ||
		strings.Contains(lower, "network") || strings.Contains(lower, "api")
}

func isWriteTool(toolName string) bool {
	lower := strings.ToLower(toolName)
	return strings.Contains(lower, "write") || strings.Contains(lower, "create") ||
		strings.Contains(lower, "save") || strings.Contains(lower, "upload") ||
		strings.Contains(lower, "append") || strings.Contains(lower, "put")
}
