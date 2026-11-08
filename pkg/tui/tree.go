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
	orderedPins, _ := hub.LoadPinnedConversations()
	return BuildProjectGroupsWithProjectsAndPins(conversations, expandedState, hub.LoadAllProjectNames(), orderedPins)
}

// BuildProjectGroupsWithProjects allows injecting known project names without pins.
func BuildProjectGroupsWithProjects(conversations []hub.ConversationItem, expandedState map[string]bool, knownProjects []string) ([]*ProjectGroup, []SidebarNode) {
	return BuildProjectGroupsWithProjectsAndPins(conversations, expandedState, knownProjects, nil)
}

// BuildProjectGroupsWithProjectsAndPins allows injecting known project names and pinned conversation IDs.
func BuildProjectGroupsWithProjectsAndPins(conversations []hub.ConversationItem, expandedState map[string]bool, knownProjects []string, pinnedOrder []string) ([]*ProjectGroup, []SidebarNode) {
	groupMap := make(map[string]*ProjectGroup)
	convByID := make(map[string]hub.ConversationItem)

	// Pre-populate known projects from configuration
	for _, projName := range knownProjects {
		isExpanded := false
		if val, ok := expandedState[projName]; ok {
			isExpanded = val
		}
		groupMap[projName] = &ProjectGroup{
			Name:     projName,
			Expanded: isExpanded,
		}
	}

	for _, conv := range conversations {
		convByID[conv.CascadeID] = conv
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
		// Sort conversations within group: pinned conversations first, then newest first
		sort.Slice(g.Conversations, func(i, j int) bool {
			if g.Conversations[i].Pinned != g.Conversations[j].Pinned {
				return g.Conversations[i].Pinned
			}
			return g.Conversations[i].LastModifiedTime > g.Conversations[j].LastModifiedTime
		})
		groups = append(groups, g)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].NewestTime == groups[j].NewestTime {
			return groups[i].Name < groups[j].Name
		}
		return groups[i].NewestTime > groups[j].NewestTime
	})

	// Add special PINNED section at the top if there are pinned conversations
	var pinnedConvs []hub.ConversationItem
	for _, pid := range pinnedOrder {
		if c, ok := convByID[pid]; ok {
			c.Pinned = true
			pinnedConvs = append(pinnedConvs, c)
		}
	}

	if len(pinnedConvs) > 0 {
		isPinnedExpanded := true // Default to expanded so pinned chats are immediately accessible
		if val, ok := expandedState["PINNED"]; ok {
			isPinnedExpanded = val
		}
		pinnedGroup := &ProjectGroup{
			Name:          "PINNED",
			Expanded:      isPinnedExpanded,
			Conversations: pinnedConvs,
			NewestTime:    "9999-99-99T99:99:99Z",
		}
		groups = append([]*ProjectGroup{pinnedGroup}, groups...)
	}

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
