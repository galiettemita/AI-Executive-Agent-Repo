// Package subagent implements the SubAgentOrchestrator task decomposition system.
package subagent

import (
	"fmt"
	"sort"
	"strings"
)

// Domain classifies a workload domain for parallel scheduling.
type Domain string

const (
	DomainResearch    Domain = "research"
	DomainWrite       Domain = "write"
	DomainSchedule    Domain = "schedule"
	DomainCommunicate Domain = "communicate"
	DomainTasks       Domain = "tasks"
	DomainUnknown     Domain = "unknown"
)

var toolDomainPrefixes = []struct {
	prefix string
	domain Domain
}{
	{"web.", DomainResearch}, {"crm.", DomainResearch}, {"drive.search", DomainResearch},
	{"email.draft", DomainWrite}, {"email.send", DomainWrite}, {"email.reply", DomainWrite},
	{"email.forward", DomainWrite}, {"email.compose", DomainWrite},
	{"drive.create", DomainWrite}, {"drive.write", DomainWrite},
	{"calendar.", DomainSchedule}, {"gcal.", DomainSchedule},
	{"slack.", DomainCommunicate},
	{"tasks.", DomainTasks},
}

// ClassifyToolKey maps a tool key to its Domain.
func ClassifyToolKey(toolKey string) Domain {
	lower := strings.ToLower(toolKey)
	for _, entry := range toolDomainPrefixes {
		if lower == entry.prefix || strings.HasPrefix(lower, entry.prefix) {
			return entry.domain
		}
	}
	return DomainUnknown
}

// SubTask is a group of tool keys that execute as one child workflow.
type SubTask struct {
	ID       string   `json:"id"`
	Domain   Domain   `json:"domain"`
	ToolKeys []string `json:"tool_keys"`
	Intent   string   `json:"intent"`
	Priority int      `json:"priority"`
}

// DecompositionResult describes how a tool list was partitioned.
type DecompositionResult struct {
	CanParallelize bool      `json:"can_parallelize"`
	SubTasks       []SubTask `json:"sub_tasks"`
	OriginalIntent string    `json:"original_intent"`
	Reason         string    `json:"reason"`
}

// Decompose splits a list of tool keys into parallel-eligible sub-task groups.
func Decompose(intent string, toolKeys []string) DecompositionResult {
	if len(toolKeys) < 2 {
		return DecompositionResult{
			CanParallelize: false, OriginalIntent: intent,
			Reason: fmt.Sprintf("only %d tool key(s) — no parallelisation benefit", len(toolKeys)),
		}
	}

	domainGroups := make(map[Domain][]string)
	for _, tk := range toolKeys {
		d := ClassifyToolKey(tk)
		domainGroups[d] = append(domainGroups[d], tk)
	}

	if len(domainGroups) < 2 {
		return DecompositionResult{
			CanParallelize: false, OriginalIntent: intent,
			Reason: "all tools in the same domain — sequential execution preferred",
		}
	}

	_, hasResearch := domainGroups[DomainResearch]

	priorityFor := func(d Domain) int {
		switch d {
		case DomainResearch:
			return 0
		case DomainWrite:
			if hasResearch {
				return 2
			}
			return 1
		default:
			return 1
		}
	}

	var subTasks []SubTask
	idx := 0
	for d, keys := range domainGroups {
		subTasks = append(subTasks, SubTask{
			ID: fmt.Sprintf("sub-%d", idx), Domain: d, ToolKeys: keys,
			Intent: string(d) + " operations: " + intent, Priority: priorityFor(d),
		})
		idx++
	}

	sort.Slice(subTasks, func(i, j int) bool {
		if subTasks[i].Priority != subTasks[j].Priority {
			return subTasks[i].Priority < subTasks[j].Priority
		}
		return string(subTasks[i].Domain) < string(subTasks[j].Domain)
	})

	_, hasWrite := domainGroups[DomainWrite]
	reason := fmt.Sprintf("%d independent domain groups (research=%v, write=%v, research→write dependency=%v)",
		len(subTasks), hasResearch, hasWrite, hasResearch && hasWrite)

	return DecompositionResult{
		CanParallelize: len(subTasks) >= 2, SubTasks: subTasks,
		OriginalIntent: intent, Reason: reason,
	}
}

// SplitByPriority partitions sub-tasks into execution buckets by priority value.
func SplitByPriority(subTasks []SubTask) [][]SubTask {
	if len(subTasks) == 0 {
		return nil
	}
	maxPri := 0
	for _, st := range subTasks {
		if st.Priority > maxPri {
			maxPri = st.Priority
		}
	}
	buckets := make([][]SubTask, maxPri+1)
	for _, st := range subTasks {
		buckets[st.Priority] = append(buckets[st.Priority], st)
	}
	return buckets
}
