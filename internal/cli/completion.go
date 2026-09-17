package cli

import (
	"fmt"
	"io"
)

const commandNames = "init new view edit baseline refine diff commit apply log status restore config update doctor spec check completion"

func runCompletion(args []string, out, errOut io.Writer) int {
	if len(args) != 1 {
		return fail(errOut, fmt.Errorf("usage: respex completion <bash|zsh|fish|powershell>"))
	}
	switch args[0] {
	case "bash":
		fmt.Fprintf(out, "complete -W '%s' respex\n", commandNames)
	case "zsh":
		fmt.Fprintf(out, "compdef '_arguments \"1:command:(%s)\"' respex\n", commandNames)
	case "fish":
		fmt.Fprintf(out, "complete -c respex -f -a '%s'\n", commandNames)
	case "powershell":
		fmt.Fprintf(out, "Register-ArgumentCompleter -Native -CommandName respex -ScriptBlock { param($wordToComplete) '%s'.Split(' ') | Where-Object { $_ -like \"$wordToComplete*\" } }\n", commandNames)
	default:
		return fail(errOut, fmt.Errorf("usage: respex completion <bash|zsh|fish|powershell>"))
	}
	return 0
}
