package cli

import "fmt"

type Command interface {
	Name() string
	Run(args []string, env Env) int
}

var commands = &Registry{commands: make(map[string]Command)}

type Registry struct {
	commands map[string]Command
	ordered  []string
}

func (r *Registry) Register(cmd Command) {
	name := cmd.Name()
	r.commands[name] = cmd
	r.ordered = append(r.ordered, name)
}

func (r *Registry) Names() []string {
	return r.ordered
}

func (r *Registry) Has(name string) bool {
	_, ok := r.commands[name]
	return ok
}

func (r *Registry) Dispatch(args []string, env Env) int {
	if len(args) == 0 {
		printUsage(env.Stderr)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			fmt.Fprintf(env.Stderr, "%s accepts no arguments\n", args[0])
			return 2
		}
		printUsage(env.Stdout)
		return 0
	default:
		if cmd, ok := r.commands[args[0]]; ok {
			return cmd.Run(args[1:], env)
		}
		fmt.Fprintf(env.Stderr, "unknown command %q\n", args[0])
		printUsage(env.Stderr)
		return 2
	}
}
