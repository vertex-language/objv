package cli

import (
	"flag"
	"fmt"
	"io"
)

// cmdEnv prints the resolved configuration: include/framework search lists,
// SDK, deployment target, and (with -defines) predefined macros.
func cmdEnv(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	defines := fs.Bool("defines", false, "also print the predefined macros")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "objv env: takes no files")
		return exitUsage
	}

	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}
	env, err := c.Env()
	if err != nil {
		fmt.Fprintln(stderr, "objv:", err)
		return exitUsage
	}

	fmt.Fprintf(stdout, "target: %s\nhosted: %v\narc: %v\n",
		env.Target.Name(), env.Hosted, env.ARC)
	if env.SDK.Found() {
		fmt.Fprintf(stdout, "sdk: %s\nsdk version: %s\n", env.SDK.Path, env.SDK.Version)
	}
	if !env.Deployment.IsZero() {
		fmt.Fprintf(stdout, "deployment target: %s (__ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__ %d)\n",
			env.Deployment, env.Deployment.MacroValue())
	}

	fmt.Fprintln(stdout, "\nsearch:")
	for _, m := range env.Search {
		fmt.Fprintf(stdout, "  %s%s\n", m.Name, systemTag(m.System))
	}
	if len(env.Frameworks) > 0 {
		fmt.Fprintln(stdout, "\nframeworks:")
		for _, m := range env.Frameworks {
			fmt.Fprintf(stdout, "  %s%s\n", m.Name, systemTag(m.System))
		}
	}
	if len(env.LibraryDirs) > 0 {
		fmt.Fprintln(stdout, "\nlibrary directories:")
		for _, dir := range env.LibraryDirs {
			fmt.Fprintf(stdout, "  %s\n", dir)
		}
	}
	if len(env.Libraries) > 0 {
		fmt.Fprintln(stdout, "\nlinked by default:")
		for _, l := range env.Libraries {
			fmt.Fprintf(stdout, "  -l%s\n", l)
		}
	}
	for _, n := range env.Notes {
		fmt.Fprintf(stdout, "\nnote: %s\n", n)
	}

	if *defines {
		fmt.Fprintln(stdout, "\npredefines:")
		for _, d := range env.Predefines {
			fmt.Fprintf(stdout, "  -D %s\n", d.Text)
		}
	}
	return exitOK
}

func systemTag(system bool) string {
	if system {
		return "  [system]"
	}
	return ""
}
