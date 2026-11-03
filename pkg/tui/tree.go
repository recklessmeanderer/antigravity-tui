package tui

import (
	"net/url"
	"sort"
	"strings"

	"antigravity-tui/pkg/hub"
)

type ProjectGroup struct {
	Name          string
	Expanded      bool
	Conversations []hub.ConversationItem
	NewestTime    string
}

type SidebarNode struct {
	IsHeader     bool
	ProjectName  string
	Count        int
	Conversation *hub.ConversationItem
}

// BuildProjectGroups groups conversations by project, applies collapse/expand state,
// and flattens visible items into a navigable list of SidebarNodes.
func BuildProjectGroups(conversations []hub.ConversationItem, expandedState map[string]bool) ([]*ProjectGroup, []SidebarNode) {
	groupMap := make(map[string]*ProjectGroup)

	for _, conv := range conversations {
		projName := strings.TrimSpace(conv.WorkspaceName)
		if projName == "" {
			projName = "General"
		} else {
			if unescaped, err := url.PathUnescape(projName); err == nil {
				projName = unescaped
			}
		}

		group, exists := groupMap[projName]
		if !exists {
			// All start collapsed (false) unless explicitly expanded by user
			isExpanded := false
			if val, ok := expandedState[projName]; ok {
				isExpanded = val
			}
			group = &ProjectGroup{
				Name:          projName,
				Expanded:      isExpanded,
				Conversations: nil,
			}
			groupMap[projName] = group
		}

		group.Conversations = append(group.Conversations, conv)
		if conv.LastModifiedTime > group.NewestTime {
			group.NewestTime = conv.LastModifiedTime
		}
	}

	// Collect groups and sort newest first
	var groups []*ProjectGroup
	for _, g := range groupMap {
		// Sort conversations within group newest first
		sort.Slice(g.Conversations, func(i, j int) bool {
			return g.Conversations[i].LastModifiedTime > g.Conversations[j].LastModifiedTime
		})
		groups = append(groups, g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].NewestTime > groups[j].NewestTime
	})

	// Flatten visible nodes
	var visibleNodes []SidebarNode
	for _, g := range groups {
		// Header node
		visibleNodes = append(visibleNodes, SidebarNode{
			IsHeader:    true,
			ProjectName: g.Name,
			Count:       len(g.Conversations),
		})

		// If expanded, add conversation items
		if g.Expanded {
			for i := range g.Conversations {
				visibleNodes = append(visibleNodes, SidebarNode{
					IsHeader:     false,
					ProjectName:  g.Name,
					Conversation: &g.Conversations[i],
				})
			}
		}
	}

	return groups, visibleNodes
}
