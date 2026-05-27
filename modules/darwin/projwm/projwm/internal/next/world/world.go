package world

import (
	"fmt"
	"path/filepath"
	"sort"
)

type Kind string

const (
	KindAI      Kind = "ai"
	KindShell   Kind = "shell"
	KindEditor  Kind = "editor"
	KindBrowser Kind = "browser"
)

type AI string

const (
	AIClaude  AI = "claude"
	AICopilot AI = "copilot"
)

type DesiredWorld struct {
	ActiveProfile string
	Profiles      map[string]Profile
	Projects      map[string]Project
}

type Profile struct {
	Assignments map[string]string
}

type Project struct {
	CWD      string
	Archived bool
	Windows  []Window
}

type Window struct {
	ID             int
	Kind           Kind
	AI             AI
	BrowserProfile string
}

func NewDesiredWorld() *DesiredWorld {
	return &DesiredWorld{
		Profiles: map[string]Profile{},
		Projects: map[string]Project{},
	}
}

func Validate(w *DesiredWorld) error {
	if w.ActiveProfile != "" {
		if _, ok := w.Profiles[w.ActiveProfile]; !ok {
			return fmt.Errorf("active profile %q does not exist", w.ActiveProfile)
		}
	}
	for profileName, profile := range w.Profiles {
		seenProjects := map[string]string{}
		for slot, projectName := range profile.Assignments {
			project, ok := w.Projects[projectName]
			if !ok {
				return fmt.Errorf("profile %q slot %q assigns unknown project %q", profileName, slot, projectName)
			}
			if profileName == w.ActiveProfile && project.Archived {
				return fmt.Errorf("active profile %q contains archived project %q", profileName, projectName)
			}
			if otherSlot, ok := seenProjects[projectName]; ok {
				return fmt.Errorf("profile %q assigns project %q to multiple slots %q and %q", profileName, projectName, otherSlot, slot)
			}
			seenProjects[projectName] = slot
		}
	}
	for projectName, project := range w.Projects {
		seenWindows := map[string]bool{}
		for _, win := range project.Windows {
			if !IsValidKind(win.Kind) {
				return fmt.Errorf("project %q has invalid kind %q", projectName, win.Kind)
			}
			if win.ID < 1 {
				return fmt.Errorf("project %q has invalid window id %d", projectName, win.ID)
			}
			key := fmt.Sprintf("%s-%d", win.Kind, win.ID)
			if seenWindows[key] {
				return fmt.Errorf("project %q has duplicate window %s", projectName, key)
			}
			seenWindows[key] = true
			if win.Kind == KindAI {
				if !IsValidAI(win.AI) {
					return fmt.Errorf("project %q window %s requires valid ai", projectName, key)
				}
				continue
			}
			if win.AI != "" {
				return fmt.Errorf("project %q window %s has ai field but is not ai kind", projectName, key)
			}
			if win.Kind != KindBrowser && win.BrowserProfile != "" {
				return fmt.Errorf("project %q window %s has browser profile but is not browser kind", projectName, key)
			}
		}
	}
	if w.ActiveProfile != "" {
		seenBasenames := map[string]string{}
		for _, projectName := range w.Profiles[w.ActiveProfile].Assignments {
			project := w.Projects[projectName]
			if project.Archived {
				continue
			}
			base := filepath.Base(project.CWD)
			if other, ok := seenBasenames[base]; ok && other != projectName {
				return fmt.Errorf("active projects %q and %q share basename %q", other, projectName, base)
			}
			seenBasenames[base] = projectName
		}
	}
	return nil
}

func IsValidKind(k Kind) bool {
	return k == KindAI || k == KindShell || k == KindEditor || k == KindBrowser
}

func IsValidAI(ai AI) bool {
	return ai == AIClaude || ai == AICopilot
}

func NextWindowID(p Project, kind Kind) int {
	maxID := 0
	for _, w := range p.Windows {
		if w.Kind == kind && w.ID > maxID {
			maxID = w.ID
		}
	}
	return maxID + 1
}

func SortedWindows(p Project) []Window {
	out := append([]Window(nil), p.Windows...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func IsParked(w *DesiredWorld, projectName string) bool {
	project, ok := w.Projects[projectName]
	if !ok || project.Archived {
		return false
	}
	for _, profile := range w.Profiles {
		for _, assigned := range profile.Assignments {
			if assigned == projectName {
				return false
			}
		}
	}
	return true
}

func GhosttyTitle(kind Kind, id int, project string) string {
	if kind == KindEditor {
		panic("editor has no ghostty title")
	}
	return fmt.Sprintf("%s-%d:%s", kind, id, project)
}

func TmuxSession(kind Kind, id int, project string) string {
	if kind == KindEditor {
		panic("editor has no tmux session")
	}
	return fmt.Sprintf("%s-%d/%s", kind, id, project)
}

func ViewerGhosttyTitle(id int, project string) string {
	return fmt.Sprintf("ai-view-%d:%s", id, project)
}

func ViewerTmuxSession(id int, project string) string {
	return fmt.Sprintf("ai-%d/%s_v", id, project)
}

func ZedTitle(cwd string) string {
	return filepath.Base(cwd)
}

func kindOrder(k Kind) int {
	switch k {
	case KindAI:
		return 0
	case KindShell:
		return 1
	case KindEditor:
		return 2
	case KindBrowser:
		return 3
	default:
		return 99
	}
}
