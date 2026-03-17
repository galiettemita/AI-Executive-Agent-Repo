package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDynamicTool_ValidateDynamicCode_Blocked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code string
	}{
		{"import_os", "import os"},
		{"import_sys", "import sys"},
		{"import_subprocess", "import subprocess"},
		{"import_socket", "import socket"},
		{"dunder_import", "__import__('os')"},
		{"exec_call", "exec('code')"},
		{"eval_call", "eval('1+1')"},
		{"open_call", "open('/etc/passwd')"},
		{"file_call", "file('/etc/passwd')"},
		{"input_call", "input('prompt')"},
		{"os_system", "os.system('ls')"},
		{"os_popen", "os.popen('ls')"},
		{"subprocess_dot", "subprocess.run(['ls'])"},
		{"shutil_dot", "shutil.copy('a','b')"},
		{"glob_dot", "glob.glob('*')"},
		{"pathlib_dot", "pathlib.Path('.')"},
		{"tempfile_dot", "tempfile.mktemp()"},
		{"pickle_dot", "pickle.loads(b'')"},
		{"marshal_dot", "marshal.loads(b'')"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateDynamicCode(tc.code); err == nil {
				t.Errorf("validateDynamicCode(%q): expected error for blocked pattern, got nil", tc.code)
			}
		})
	}
}

func TestDynamicTool_ValidateDynamicCode_BlockedCaseInsensitive(t *testing.T) {
	t.Parallel()

	cases := []string{
		"Import Os",
		"IMPORT OS",
		"IMPORT SYS",
		"Import Subprocess",
		"EXEC('code')",
		"EVAL('x')",
		"OPEN('/etc')",
		"OS.SYSTEM('ls')",
		"SUBPROCESS.RUN([])",
	}
	for _, code := range cases {
		code := code
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			if err := validateDynamicCode(code); err == nil {
				t.Errorf("validateDynamicCode(%q): case-insensitive check missed blocked pattern", code)
			}
		})
	}
}

func TestDynamicTool_ValidateDynamicCode_Allowed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code string
	}{
		{
			"plan_s7_example",
			"import json\ndef execute(d): return {\"sum\":d[\"a\"]+d[\"b\"]}",
		},
		{"math_import", "import math\ndef execute(input_data: dict) -> dict:\n    return {\"sqrt\": math.sqrt(input_data[\"n\"])}"},
		{"re_import", "import re\ndef execute(input_data: dict) -> dict:\n    return {\"match\": bool(re.search(input_data[\"p\"], input_data[\"t\"]))}"},
		{"datetime_import", "import datetime\ndef execute(input_data: dict) -> dict:\n    return {\"now\": datetime.datetime.utcnow().isoformat()}"},
		{"requests_import", "import requests\ndef execute(input_data: dict) -> dict:\n    r = requests.get(input_data[\"url\"])\n    return {\"status\": r.status_code}"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateDynamicCode(tc.code); err != nil {
				t.Errorf("validateDynamicCode allowed code got unexpected error: %v\ncode: %s", err, tc.code)
			}
		})
	}
}

func TestDynamicTool_StructFields(t *testing.T) {
	t.Parallel()
	tool := DynamicTool{
		ID: "dyn_123", Name: "test_tool", Description: "A test tool",
		Code: "def execute(d): return {}", CreatedFor: "test task",
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}
	if tool.ID == "" {
		t.Error("DynamicTool.ID must be settable")
	}
	if !tool.ExpiresAt.After(time.Now()) {
		t.Error("DynamicTool.ExpiresAt: 2h expiry must be in the future")
	}
}

func TestDynamicToolResult_StructFields(t *testing.T) {
	t.Parallel()
	result := DynamicToolResult{Output: map[string]any{"key": "value"}, ExitCode: 0, Error: "", LatencyMs: 42}
	if result.Output == nil {
		t.Error("DynamicToolResult.Output must be settable")
	}
	if result.LatencyMs != 42 {
		t.Error("DynamicToolResult.LatencyMs must be settable as int64")
	}
}

func TestDynamicTool_Execute_RejectsExpiredTool(t *testing.T) {
	t.Parallel()
	expired := &DynamicTool{ID: "dyn_expired", Name: "expired_tool", Code: "def execute(d): return {}", ExpiresAt: time.Now().Add(-1 * time.Minute)}
	creator := &DynamicToolCreator{}
	_, err := creator.Execute(context.Background(), expired, map[string]any{})
	if err == nil {
		t.Fatal("Execute() on expired tool: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("Execute() expired tool error should mention 'expired', got: %v", err)
	}
}

func TestDynamicTool_Execute_RejectsNilTool(t *testing.T) {
	t.Parallel()
	creator := &DynamicToolCreator{}
	_, err := creator.Execute(context.Background(), nil, map[string]any{})
	if err == nil {
		t.Fatal("Execute() with nil tool: expected error, got nil")
	}
}

func TestDynamicTool_Execute_PythonSubprocess(t *testing.T) {
	tool := &DynamicTool{
		ID: "dyn_add_test", Name: "add_numbers", Description: "Adds two numbers",
		Code: `import json
def execute(input_data: dict) -> dict:
    try:
        return {"result": input_data["a"] + input_data["b"]}
    except Exception as e:
        return {"error": str(e)}`,
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}
	creator := &DynamicToolCreator{}
	result, err := creator.Execute(context.Background(), tool, map[string]any{"a": 3, "b": 4})
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if result.Output == nil {
		t.Fatal("Execute() result.Output is nil")
	}
	got, ok := result.Output["result"]
	if !ok {
		t.Fatalf("Execute() output missing 'result' key; output: %v", result.Output)
	}
	if got.(float64) != 7 {
		t.Errorf("Execute() result.Output[result] = %v, want 7", got)
	}
	if result.ExitCode != 0 {
		t.Errorf("Execute() ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestDynamicTool_CreateTool_EmptyDescriptionError(t *testing.T) {
	t.Parallel()
	creator := &DynamicToolCreator{llmClient: nil}
	_, err := creator.CreateTool(context.Background(), "")
	if err == nil {
		t.Fatal("CreateTool() empty taskDescription: expected error, got nil")
	}
}

func TestDynamicTool_CreateTool_WhitespaceDescriptionError(t *testing.T) {
	t.Parallel()
	creator := &DynamicToolCreator{llmClient: nil}
	_, err := creator.CreateTool(context.Background(), "   ")
	if err == nil {
		t.Fatal("CreateTool() whitespace taskDescription: expected error, got nil")
	}
}
