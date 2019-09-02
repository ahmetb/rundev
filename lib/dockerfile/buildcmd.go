package dockerfile

import (
	"github.com/ahmetb/rundev/lib/types"
	"regexp"
	"strings"
	"unicode"
)

var (
	runCmdAnnotationPattern = regexp.MustCompile(`\s+#\s?rundev(\[(.*)\])?$`)
)

// ParseBuildCmds extracts RUN commands from the last dockerfile stage with #rundev or #rundev[PATTERN,..] annotation.
func ParseBuildCmds(d *Dockerfile) types.BuildCmds {
	var out types.BuildCmds
	for _, stmt := range d.Stmts() {
		switch stmt.Value {
		case "from":
			out = nil // reset
		case "run":
			if !runCmdAnnotationPattern.MatchString(stmt.Original) {
				continue
			}

			var conditions []string
			parts := runCmdAnnotationPattern.FindStringSubmatch(stmt.Original)
			if len(parts) > 1 {
				condStr := parts[len(parts)-1]
				conds := strings.Split(condStr, ",")
				if condStr != "" && len(conds) > 0 {
					conditions = conds
				}
			}

			c := parseCommand(stmt.Next, stmt.Attributes["json"])
			cm := Cmd{c[0], c[1:]}

			// trim comment at the end of argv (as dockerfile parser isn't doing so)
			if len(cm.Args) > 0 {
				v := cm.Args[len(cm.Args)-1]
				v = runCmdAnnotationPattern.ReplaceAllString(v, "")
				v = strings.TrimRightFunc(v, unicode.IsSpace)
				cm.Args[len(cm.Args)-1] = v
			}
			out = append(out, types.BuildCmd{
				C:  cm.Flatten(),
				On: conditions,
			})
		}
	}
	return out
}
