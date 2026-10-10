// Copyright 2019 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	tui "charm.land/lipgloss/v2"
	"github.com/minio/kes/internal/cli"
	"github.com/minio/kms-go/kes"
	flag "github.com/spf13/pflag"
)

const policyCmdUsage = `Usage:
    kes policy <command>

Commands:
    info                     Get information about a policy.
    ls                       List policies.
    show                     Display a policy.

Options:
    -h, --help               Print command line options.
`

func policyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, policyCmdUsage) }

	subCmds := commands{
		"info": infoPolicyCmd,
		"ls":   lsPolicyCmd,
		"show": showPolicyCmd,
	}
	if len(args) < 2 {
		cmd.Usage()
		os.Exit(2)
	}
	if cmd, ok := subCmds[args[1]]; ok {
		cmd(args[1:])
		return
	}
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes policy --help'", err)
	}
	if cmd.NArg() > 0 {
		cli.Fatalf("%q is not a policy command. See 'kes policy --help'", cmd.Arg(0))
	}
	cmd.Usage()
	os.Exit(2)
}

const lsPolicyCmdUsage = `Usage:
    kes policy ls [options] [<pattern>]

Options:
        --json               Print policies in JSON format.

    -h, --help               Print command line options.

Examples:
    $ kes policy ls
    $ kes policy ls 'my-policy*'
`

func lsPolicyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, lsPolicyCmdUsage) }

	var (
		jsonFlag bool
	)
	cmd.BoolVar(&jsonFlag, "json", false, "Print identities in JSON format")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes policy ls --help'", err)
	}

	if cmd.NArg() > 1 {
		cli.Fatal("too many arguments. See 'kes policy ls --help'")
	}

	prefix := "*"
	if cmd.NArg() == 1 {
		prefix = cmd.Arg(0)
	}

	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelCtx()

	enclave := newClient(config{})
	iter := &kes.ListIter[string]{
		NextFunc: enclave.ListPolicies,
	}

	var names []string
	for id, err := iter.SeekTo(ctx, prefix); err != io.EOF; id, err = iter.Next(ctx) {
		if err != nil {
			cli.Fatalf("failed to list keys: %v", err)
		}
		names = append(names, id)
	}

	if jsonFlag {
		if err := json.NewEncoder(os.Stdout).Encode(names); err != nil {
			cli.Fatalf("failed to list keys: %v", err)
		}
	}
	if len(names) == 0 {
		return
	}

	var (
		style = tui.NewStyle().Underline(cli.IsTerminal())
		buf   = &strings.Builder{}
	)
	tui.Fprintln(buf, style.Render("Key"))
	for _, name := range names {
		buf.WriteString(name)
		buf.WriteByte('\n')
	}
	tui.Print(buf)
}

const infoPolicyCmdUsage = `Usage:
    kes policy info [options] <name>

Options:
        --json               Print policy in JSON format.
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Print command line options.

Examples:
    $ kes policy info my-policy
`

func infoPolicyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, infoPolicyCmdUsage) }

	var (
		jsonFlag    bool
		enclaveName string
	)
	cmd.BoolVar(&jsonFlag, "json", false, "Print policy in JSON format.")
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes policy show --help'", err)
	}
	if cmd.NArg() == 0 {
		cli.Fatal("no policy name specified. See 'kes policy show --help'")
	}

	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelCtx()

	name := cmd.Arg(0)
	client := newClient(config{})
	info, err := client.DescribePolicy(ctx, name)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		cli.Fatal(err)
	}
	if jsonFlag {
		encoder := json.NewEncoder(os.Stdout)
		if cli.IsTerminal() {
			encoder.SetIndent("", "  ")
		}
		if err = encoder.Encode(info); err != nil {
			cli.Fatal(err)
		}
	} else {
		var faint, policyStyle tui.Style
		if cli.IsTerminal() {
			ColorPolicy := tui.Color("#2e42d1")
			faint = faint.Faint(true)
			policyStyle = policyStyle.Foreground(ColorPolicy)
		}
		tui.Println(faint.Render(fmt.Sprintf("%-11s", "Name")), policyStyle.Render(name))
		if !info.CreatedAt.IsZero() {
			year, month, day := info.CreatedAt.Local().Date()
			hour, minute, sec := info.CreatedAt.Local().Clock()
			tui.Println(
				faint.Render(fmt.Sprintf("%-11s", "Date")),
				fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, month, day, hour, minute, sec),
			)
		}
		if !info.CreatedBy.IsUnknown() {
			tui.Println(faint.Render(fmt.Sprintf("%-11s", "Created by")), info.CreatedBy)
		}
	}
}

const showPolicyCmdUsage = `Usage:
    kes policy show [options] <name>

Options:
    -e, --enclave <name>     Operate within the specified enclave.
        --json               Print policy in JSON format.

    -h, --help               Print command line options.

Examples:
    $ kes policy show my-policy
`

func showPolicyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, showPolicyCmdUsage) }

	var (
		jsonFlag    bool
		enclaveName string
	)
	cmd.BoolVar(&jsonFlag, "json", false, "Print policy in JSON format.")
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes policy show --help'", err)
	}
	if cmd.NArg() == 0 {
		cli.Fatal("no policy name specified. See 'kes policy show --help'")
	}

	name := cmd.Arg(0)
	client := newClient(config{})

	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelCtx()

	policy, err := client.GetPolicy(ctx, name)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		cli.Fatalf("failed to show policy '%s': %v", name, err)
	}
	if !cli.IsTerminal() || jsonFlag {
		type Response struct {
			Allow     map[string]kes.Rule `json:"allow,omitempty"`
			Deny      map[string]kes.Rule `json:"deny,omitempty"`
			CreatedAt time.Time           `json:"created_at,omitempty"`
			CreatedBy kes.Identity        `json:"created_by,omitempty"`
		}
		encoder := json.NewEncoder(os.Stdout)
		if cli.IsTerminal() {
			encoder.SetIndent("", "  ")
		}
		err = encoder.Encode(Response{
			Allow:     policy.Allow,
			Deny:      policy.Deny,
			CreatedAt: policy.CreatedAt,
			CreatedBy: policy.CreatedBy,
		})
		if err != nil {
			cli.Fatalf("failed to show policy '%s': %v", name, err)
		}
	} else {
		var (
			Red   = tui.Color("#d70000")
			Green = tui.Color("#00a700")
			Cyan  = tui.Color("#00afaf")
		)
		if len(policy.Allow) > 0 {
			header := tui.NewStyle().Bold(true).Foreground(Green)
			tui.Println(header.Render("Allow:"))
			for rule := range policy.Allow {
				tui.Println("  · " + rule)
			}
		}
		if len(policy.Deny) > 0 {
			if len(policy.Allow) > 0 {
				tui.Println()
			}
			header := tui.NewStyle().Bold(true).Foreground(Red)
			tui.Println(header.Render("Deny:"))
			for rule := range policy.Deny {
				tui.Println("  · " + rule)
			}
		}

		tui.Println()
		header := tui.NewStyle().Bold(true).Foreground(Cyan)
		if !policy.CreatedAt.IsZero() {
			year, month, day := policy.CreatedAt.Local().Date()
			hour, minute, sec := policy.CreatedAt.Local().Clock()
			tui.Printf("\n%s %04d-%02d-%02d %02d:%02d:%02d\n", header.Render("Created at:"), year, month, day, hour, minute, sec)
		}
		if !policy.CreatedBy.IsUnknown() {
			tui.Println(header.Render("Created by:"), policy.CreatedBy)
		} else {
			tui.Println(header.Render("Created by:"), "<unknown>")
		}
	}
}
