package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/davis7dotsh/bgst/internal/cli"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	resolveBuildInfo()
	app := cli.New(os.Stdin, os.Stdout, cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})

	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "bgst: %v\n", err)
		os.Exit(1)
	}
}

func resolveBuildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if (version == "" || version == "dev") && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if commit == "" {
				commit = setting.Value
				if len(commit) > 7 {
					commit = commit[:7]
				}
			}
		case "vcs.time":
			if date == "" {
				date = setting.Value
			}
		case "vcs.modified":
			if setting.Value == "true" && !strings.HasSuffix(version, "+dirty") {
				version += "+dirty"
			}
		}
	}
}
