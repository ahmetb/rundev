package dockerfile

import (
	"bytes"
	"fmt"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/pkg/errors"
)

type Dockerfile struct {
	syntaxTree *parser.Node
}

func (d *Dockerfile) Stmts() []*parser.Node { return d.syntaxTree.Children }

type Cmd struct {
	Cmd  string
	Args []string
}

func (c Cmd) String() string {
	s := c.Cmd
	for _, a := range c.Args {
		s += fmt.Sprintf(" %q", a)
	}
	return s
}

func (c Cmd) Flatten() []string {
	if c.Cmd == "" {
		return nil
	}
	return append([]string{c.Cmd}, c.Args...)
}

func ParseDockerfile(b []byte) (*Dockerfile, error) {
	r, err := parser.Parse(bytes.NewReader(b))
	if err != nil {
		return nil, errors.Wrap(err, "error parsing dockerfile")
	}
	if r.AST == nil {
		return nil, errors.Wrap(err, "ast was nil")
	}
	return &Dockerfile{r.AST}, nil
}

func ParseEntrypoint(d *Dockerfile) (Cmd, error) {
	var c Cmd
	var epVals, cmdVals []string
	for _, stmt := range d.Stmts() {
		switch stmt.Value {
		case "from":
			// reset (new stage)
			epVals = nil
			cmdVals = nil
		case "entrypoint":
			epVals = parseCommand(stmt.Next, stmt.Attributes["json"])
		case "cmd":
			cmdVals = parseCommand(stmt.Next, stmt.Attributes["json"])
		}
	}
	if len(epVals) == 0 && len(cmdVals) == 0 {
		return c, errors.New("no CMD or ENTRYPOINT values in dockerfile")
	}
	if len(epVals) == 0 {
		// CMD becomes the entrypoint
		return Cmd{cmdVals[0], cmdVals[1:]}, nil
	}
	// merge ENTRYPOINT argv and CMD values
	return Cmd{epVals[0], append(epVals[1:], cmdVals...)}, nil
}

// parseCommand parses CMD and ENTRYPOINT nodes, based on whether they're JSON lists or not.
// Non-JSON values are wrapped in [/bin/sh -c VALUE]
func parseCommand(n *parser.Node, json bool) []string {
	var out []string
	for n != nil {
		if !json {
			return []string{"/bin/sh", "-c", n.Value}
		}
		out = append(out, n.Value)
		n = n.Next
	}
	return out
}
