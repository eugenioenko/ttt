package command

import "sort"

type Command struct {
	ID       string
	Title    string
	Shortcut string
	Keywords []string
	Handler  func()
	Hidden   bool
}

type Registry struct {
	commands map[string]Command
}

func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

func (r *Registry) Register(cmd Command) {
	r.commands[cmd.ID] = cmd
}

func (r *Registry) Execute(id string) bool {
	cmd, ok := r.commands[id]
	if !ok {
		return false
	}
	cmd.Handler()
	return true
}

func (r *Registry) Get(id string) (Command, bool) {
	cmd, ok := r.commands[id]
	return cmd, ok
}

func (r *Registry) SetShortcut(id, shortcut string) {
	if cmd, ok := r.commands[id]; ok {
		cmd.Shortcut = shortcut
		r.commands[id] = cmd
	}
}

func (r *Registry) ClearAllShortcuts() {
	for id, cmd := range r.commands {
		cmd.Shortcut = ""
		r.commands[id] = cmd
	}
}

func (r *Registry) FindByTitle(title string) (Command, bool) {
	for _, cmd := range r.commands {
		if cmd.Title == title {
			return cmd, true
		}
	}
	return Command{}, false
}

func (r *Registry) List() []Command {
	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		if cmd.Hidden {
			continue
		}
		cmds = append(cmds, cmd)
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Title < cmds[j].Title
	})
	return cmds
}
