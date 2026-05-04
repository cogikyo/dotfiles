package cli

import (
	"fmt"
	"io"

	"dotfiles/dctl/internal/execx"
	"dotfiles/dctl/internal/prompt"
	"dotfiles/dctl/internal/tui"

	"github.com/alecthomas/kong"
)

func ShouldLaunchNavigator(args []string) bool {
	return len(args) == 0 && prompt.Interactive()
}

func BuildCommandCatalog(k *kong.Kong) tui.CommandCatalog {
	if k == nil || k.Model == nil || k.Model.Node == nil {
		return tui.CommandCatalog{}
	}
	return walkKongNode(k.Model.Node)
}

func walkKongNode(n *kong.Node) tui.CommandNode {
	node := tui.CommandNode{Name: n.Name, Help: n.Help}
	if n.Group != nil {
		node.GroupKey = n.Group.Key
		node.GroupTitle = n.Group.Title
	}
	for _, child := range n.Children {
		if child == nil || child.Hidden || child.Name == "?" || child.Name == "help" {
			continue
		}
		node.Children = append(node.Children, walkKongNode(child))
	}
	return node
}

func RunNavigator(w io.Writer, catalog tui.CommandCatalog) ([]string, bool, error) {
	if len(catalog.Children) == 0 {
		return nil, false, fmt.Errorf("command catalog is empty")
	}
	return tui.RunLauncher(w, catalog)
}

func ExecSubcommand(argv []string) error {
	return execx.ExecDctl(argv)
}
