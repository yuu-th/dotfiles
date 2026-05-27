package planner

type Window struct {
	ID        string
	Kind      string
	Project   string
	Workspace string
}

type World struct {
	Windows []Window
}

type Action struct {
	Op     string
	Window Window
}

type Planner struct {
	Spawn func(Window)
}

func Diff(desired, observed World) []Action {
	seen := map[string]bool{}
	for _, w := range observed.Windows {
		seen[w.ID] = true
	}
	var actions []Action
	for _, w := range desired.Windows {
		if !seen[w.ID] {
			actions = append(actions, Action{Op: "spawn", Window: w})
		}
	}
	return actions
}

func (p Planner) Apply(actions []Action, dryRun bool) {
	if dryRun {
		return
	}
	for _, a := range actions {
		if a.Op == "spawn" && p.Spawn != nil {
			p.Spawn(a.Window)
		}
	}
}
