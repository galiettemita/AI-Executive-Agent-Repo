package temporal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestBrowserStartSession_MissingURL(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.StartBrowserSessionActivity)

	_, err := env.ExecuteActivity(acts.StartBrowserSessionActivity, BrowserSessionInput{
		WorkspaceID: "ws-1", SessionType: "scrape", URL: "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BROWSER_VALIDATION_FAILED")
}

func TestBrowserStartSession_NoBrowserClient_UsesHashID(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.StartBrowserSessionActivity)

	val, err := env.ExecuteActivity(acts.StartBrowserSessionActivity, BrowserSessionInput{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		SessionType: "scrape", URL: "https://example.com",
	})
	require.NoError(t, err)
	var result BrowserSessionResult
	require.NoError(t, val.Get(&result))
	assert.NotEmpty(t, result.SessionID)
	assert.Equal(t, "active", result.Status)
}

func TestBrowserExecuteTask_NoBrowserClient_ReturnsDescriptive(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.ExecuteBrowserTaskActivity)

	val, err := env.ExecuteActivity(acts.ExecuteBrowserTaskActivity, BrowserTaskInput{
		WorkspaceID: "ws-1", SessionID: "sess-1", SessionType: "scrape", URL: "https://example.com",
	})
	require.NoError(t, err)
	var result BrowserTaskResult
	require.NoError(t, val.Get(&result))
	assert.NotContains(t, result.Result, `"current_price":29.99`)
	assert.NotContains(t, result.Result, `"booked":true`)
	assert.Contains(t, result.Result, "browser_client_not_configured")
	assert.NotEmpty(t, result.EvidenceHash)
}

func TestBrowserExecuteTask_UnknownSessionType(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.ExecuteBrowserTaskActivity)

	_, err := env.ExecuteActivity(acts.ExecuteBrowserTaskActivity, BrowserTaskInput{
		WorkspaceID: "ws-1", SessionID: "sess-1", SessionType: "unknown_type", URL: "https://example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BROWSER_UNKNOWN_TYPE")
}

func TestBrowserCloseSession_AlwaysReturnsOK(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.CloseBrowserSessionActivity)

	val, err := env.ExecuteActivity(acts.CloseBrowserSessionActivity, BrowserCloseInput{SessionID: "sess-1"})
	require.NoError(t, err)
	var result BrowserCloseResult
	require.NoError(t, val.Get(&result))
	assert.True(t, result.Closed)
}

func TestBrowserStartSession_PublicURL_AllowedWithNoSandbox(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	acts := NewActivities()
	env.RegisterActivity(acts.StartBrowserSessionActivity)

	val, err := env.ExecuteActivity(acts.StartBrowserSessionActivity, BrowserSessionInput{
		WorkspaceID: "00000000-0000-0000-0000-000000000001",
		SessionType: "scrape", URL: "https://public.example.com",
	})
	require.NoError(t, err)
	var result BrowserSessionResult
	require.NoError(t, val.Get(&result))
	assert.Equal(t, "active", result.Status)
}
