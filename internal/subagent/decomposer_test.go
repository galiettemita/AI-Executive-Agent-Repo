package subagent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/subagent"
)

func TestClassifyToolKey_AllDomains(t *testing.T) {
	cases := []struct{ key string; want subagent.Domain }{
		{"web.search", subagent.DomainResearch}, {"crm.query", subagent.DomainResearch},
		{"drive.search", subagent.DomainResearch}, {"email.send", subagent.DomainWrite},
		{"email.draft", subagent.DomainWrite}, {"drive.create", subagent.DomainWrite},
		{"calendar.create", subagent.DomainSchedule}, {"gcal.create", subagent.DomainSchedule},
		{"slack.send", subagent.DomainCommunicate}, {"tasks.create", subagent.DomainTasks},
		{"unknown.tool", subagent.DomainUnknown},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, subagent.ClassifyToolKey(tc.key), "tool key %q", tc.key)
	}
}

func TestDecompose_TwoDomains_CanParallelize(t *testing.T) {
	result := subagent.Decompose("research and schedule", []string{"web.search", "calendar.create"})
	assert.True(t, result.CanParallelize)
	assert.Len(t, result.SubTasks, 2)
}

func TestDecompose_ResearchBeforeWrite_PriorityOrdering(t *testing.T) {
	result := subagent.Decompose("research and draft email", []string{"web.search", "email.draft"})
	require.True(t, result.CanParallelize)
	priMap := make(map[subagent.Domain]int)
	for _, st := range result.SubTasks {
		priMap[st.Domain] = st.Priority
	}
	assert.Less(t, priMap[subagent.DomainResearch], priMap[subagent.DomainWrite])
}

func TestDecompose_WriteSchedule_SamePriorityNoResearch(t *testing.T) {
	result := subagent.Decompose("send and book", []string{"email.send", "calendar.create"})
	require.True(t, result.CanParallelize)
	priMap := make(map[subagent.Domain]int)
	for _, st := range result.SubTasks {
		priMap[st.Domain] = st.Priority
	}
	assert.Equal(t, priMap[subagent.DomainWrite], priMap[subagent.DomainSchedule])
}

func TestDecompose_SingleDomain_NoParallelisation(t *testing.T) {
	result := subagent.Decompose("emails", []string{"email.send", "email.reply", "email.forward"})
	assert.False(t, result.CanParallelize)
	assert.Contains(t, result.Reason, "same domain")
}

func TestDecompose_OneTool_NoParallelisation(t *testing.T) {
	result := subagent.Decompose("create event", []string{"calendar.create"})
	assert.False(t, result.CanParallelize)
}

func TestDecompose_Empty_NoParallelisation(t *testing.T) {
	assert.False(t, subagent.Decompose("do something", nil).CanParallelize)
}

func TestSplitByPriority_ThreeBuckets(t *testing.T) {
	tasks := []subagent.SubTask{
		{ID: "a", Priority: 0}, {ID: "b", Priority: 1}, {ID: "c", Priority: 1}, {ID: "d", Priority: 2},
	}
	buckets := subagent.SplitByPriority(tasks)
	require.Len(t, buckets, 3)
	assert.Len(t, buckets[0], 1)
	assert.Len(t, buckets[1], 2)
	assert.Len(t, buckets[2], 1)
}

func TestSplitByPriority_Empty(t *testing.T) {
	assert.Nil(t, subagent.SplitByPriority(nil))
}

func TestDecompose_SubTasksSorted(t *testing.T) {
	result := subagent.Decompose("everything", []string{"email.send", "web.search", "calendar.create"})
	require.True(t, result.CanParallelize)
	for i := 1; i < len(result.SubTasks); i++ {
		assert.LessOrEqual(t, result.SubTasks[i-1].Priority, result.SubTasks[i].Priority)
	}
}
