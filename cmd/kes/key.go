// Copyright 2019 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"

	tui "charm.land/lipgloss/v2"
	"github.com/minio/kes/internal/cli"
	"github.com/minio/kms-go/kes"
	flag "github.com/spf13/pflag"
)

const keyCmdUsage = `Usage:
    kes key <command>

Commands:
    create                   Create a new crypto key.
    import                   Import a crypto key.
    info                     Get information about a crypto key. 
    ls                       List crypto keys.
    rm                       Delete a crypto key.

    encrypt                  Encrypt a message.
    decrypt                  Decrypt an encrypted message.
    dek                      Generate a new data encryption key.

Options:
    -h, --help               Print command line options.
`

func keyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, keyCmdUsage) }

	subCmds := commands{
		"create": createKeyCmd,
		"import": importKeyCmd,
		"info":   describeKeyCmd,
		"ls":     lsKeyCmd,
		"rm":     rmKeyCmd,

		"encrypt": encryptKeyCmd,
		"decrypt": decryptKeyCmd,
		"dek":     dekCmd,
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
		cli.Fatalf("%v. See 'kes key --help'", err)
	}
	if cmd.NArg() > 0 {
		cli.Fatalf("%q is not a key command. See 'kes key --help'", cmd.Arg(0))
	}
	cmd.Usage()
	os.Exit(2)
}

const createKeyCmdUsage = `Usage:
    kes key create [options] <name>...

Options:
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Print command line options.

Examples:
    $ kes key create my-key
    $ kes key create my-key1 my-key2
`

func createKeyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, createKeyCmdUsage) }

	var (
		enclaveName string
	)
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key create --help'", err)
	}

	if cmd.NArg() == 0 {
		cli.Fatal("no key name specified. See 'kes key create --help'")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	client := newClient(config{})
	for _, name := range cmd.Args() {
		if err := client.CreateKey(ctx, name); err != nil {
			if errors.Is(err, context.Canceled) {
				os.Exit(1)
			}
			cli.Fatalf("failed to create key %q: %v", name, err)
		}
	}
}

const importKeyCmdUsage = `Usage:
    kes key import [options] <name> [<key>]

Options:

    -h, --help               Print command line options.

Examples:
    $ kes key import my-key-2 Xlnr/nOgAWE5cA7GAsl3L2goCvmfs6KE0gNgB1T93wE=
`

func importKeyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, importKeyCmdUsage) }

	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key import --help'", err)
	}

	switch {
	case cmd.NArg() == 0:
		cli.Fatal("no key name specified. See 'kes key import --help'")
	case cmd.NArg() == 1:
		cli.Fatal("no crypto key specified. See 'kes key import --help'")
	case cmd.NArg() > 2:
		cli.Fatal("too many arguments. See 'kes key import --help'")
	}
	name := cmd.Arg(0)
	key, err := base64.StdEncoding.DecodeString(cmd.Arg(1))
	if err != nil {
		cli.Fatalf("invalid key: %v. See 'kes key import --help'", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	enclave := newClient(config{})
	if err = enclave.ImportKey(ctx, name, &kes.ImportKeyRequest{Key: key}); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		cli.Fatalf("failed to import %q: %v", name, err)
	}
}

const describeKeyCmdUsage = `Usage:
    kes key info [options] <name>

Options:
        --json               Print keys in JSON format. 
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Print command line options.

Examples:
    $ kes key info my-key
`

func describeKeyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, describeKeyCmdUsage) }

	var (
		jsonFlag    bool
		enclaveName string
	)
	cmd.BoolVar(&jsonFlag, "json", false, "Print identities in JSON format")
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key info --help'", err)
	}

	switch {
	case cmd.NArg() == 0:
		cli.Fatal("no key name specified. See 'kes key info --help'")
	case cmd.NArg() > 1:
		cli.Fatal("too many arguments. See 'kes key info --help'")
	}

	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelCtx()

	name := cmd.Arg(0)
	client := newClient(config{})
	info, err := client.DescribeKey(ctx, name)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		cli.Fatalf("failed to describe keys: %v", err)
	}
	if jsonFlag {
		if err = json.NewEncoder(os.Stdout).Encode(info); err != nil {
			cli.Fatalf("failed to describe keys: %v", err)
		}
		return
	}

	year, month, day := info.CreatedAt.Date()
	hour, minute, sec := info.CreatedAt.Clock()

	buf := &strings.Builder{}
	tui.Fprintf(buf, "%-11s %s\n", "Name", info.Name)
	tui.Fprintf(buf, "%-11s %s\n", "Algorithm", info.Algorithm)
	tui.Fprintf(buf, "%-11s %04d-%02d-%02d %02d:%02d:%02d\n", "Date", year, month, day, hour, minute, sec)
	tui.Fprintf(buf, "%-11s %s", "Owner", info.CreatedBy)
	tui.Print(buf)
}

const lsKeyCmdUsage = `Usage:
    kes key ls [options] [<pattern>]

Options:
        --json               Print keys in JSON format. 
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Print command line options.

Examples:
    $ kes key ls
    $ kes key ls 'my-key*'
`

func lsKeyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, lsKeyCmdUsage) }

	var (
		jsonFlag    bool
		enclaveName string
	)
	cmd.BoolVar(&jsonFlag, "json", false, "Print identities in JSON format")
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key ls --help'", err)
	}

	if cmd.NArg() > 1 {
		cli.Fatal("too many arguments. See 'kes key ls --help'")
	}

	prefix := "*"
	if cmd.NArg() == 1 {
		prefix = cmd.Arg(0)
	}

	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelCtx()

	enclave := newClient(config{})
	iter := &kes.ListIter[string]{
		NextFunc: enclave.ListKeys,
	}

	var names []string
	for id, err := iter.SeekTo(ctx, prefix); err != io.EOF; id, err = iter.Next(ctx) {
		if err != nil {
			cli.Fatalf("failed to list keys: %v", err)
		}
		names = append(names, id)
	}
	slices.Sort(names)

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

const rmKeyCmdUsage = `Usage:
    kes key rm [options] <name>...

Options:
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Show list of command-line options.

Examples:
    $ kes key rm my-key
    $ kes key rm my-key1 my-key2
`

func rmKeyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, rmKeyCmdUsage) }

	var (
		enclaveName string
	)
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key rm --help'", err)
	}
	if cmd.NArg() == 0 {
		cli.Fatal("no key name specified. See 'kes key rm --help'")
	}

	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelCtx()

	client := newClient(config{})
	for _, name := range cmd.Args() {
		if err := client.DeleteKey(ctx, name); err != nil {
			if errors.Is(err, context.Canceled) {
				os.Exit(1)
			}
			cli.Fatalf("failed to remove key %q: %v", name, err)
		}
	}
}

const encryptKeyCmdUsage = `Usage:
    kes key encrypt [options] <name> <message>

Options:
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Print command line options.

Examples:
    $ kes key encrypt my-key "Hello World"
`

func encryptKeyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, encryptKeyCmdUsage) }

	var (
		enclaveName string
	)
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key encrypt --help'", err)
	}

	switch {
	case cmd.NArg() == 0:
		cli.Fatal("no key name specified. See 'kes key encrypt --help'")
	case cmd.NArg() == 1:
		cli.Fatal("no message specified. See 'kes key encrypt --help'")
	case cmd.NArg() > 2:
		cli.Fatal("too many arguments. See 'kes key encrypt --help'")
	}

	name := cmd.Arg(0)
	message := cmd.Arg(1)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	client := newClient(config{})
	ciphertext, err := client.Encrypt(ctx, name, []byte(message), nil)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		cli.Fatalf("failed to encrypt message: %v", err)
	}

	if cli.IsTerminal() {
		tui.Printf("\nciphertext: %s\n", base64.StdEncoding.EncodeToString(ciphertext))
	} else {
		tui.Printf(`{"ciphertext":"%s"}`, base64.StdEncoding.EncodeToString(ciphertext))
	}
}

const decryptKeyCmdUsage = `Usage:
    kes key decrypt [options] <name> <ciphertext> [<context>]

Options:
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Print command line options.

Examples:
    $ CIPHERTEXT=$(kes key dek my-key | jq -r .ciphertext)
    $ kes key decrypt my-key "$CIPHERTEXT"
`

func decryptKeyCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, decryptKeyCmdUsage) }

	var (
		enclaveName string
	)
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key decrypt --help'", err)
	}

	switch {
	case cmd.NArg() == 0:
		cli.Fatal("no key name specified. See 'kes key decrypt --help'")
	case cmd.NArg() == 1:
		cli.Fatal("no ciphertext specified. See 'kes key decrypt --help'")
	case cmd.NArg() > 3:
		cli.Fatal("too many arguments. See 'kes key decrypt --help'")
	}

	name := cmd.Arg(0)
	ciphertext, err := base64.StdEncoding.DecodeString(cmd.Arg(1))
	if err != nil {
		cli.Fatalf("invalid ciphertext: %v. See 'kes key decrypt --help'", err)
	}

	var associatedData []byte
	if cmd.NArg() == 3 {
		associatedData, err = base64.StdEncoding.DecodeString(cmd.Arg(2))
		if err != nil {
			cli.Fatalf("invalid context: %v. See 'kes key decrypt --help'", err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	client := newClient(config{})
	plaintext, err := client.Decrypt(ctx, name, ciphertext, associatedData)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		cli.Fatalf("failed to decrypt ciphertext: %v", err)
	}

	if cli.IsTerminal() {
		tui.Printf("\nplaintext: %s\n", base64.StdEncoding.EncodeToString(plaintext))
	} else {
		tui.Printf(`{"plaintext":"%s"}`, base64.StdEncoding.EncodeToString(plaintext))
	}
}

const dekCmdUsage = `Usage:
    kes key dek <name> [<context>]

Options:
    -e, --enclave <name>     Operate within the specified enclave.

    -h, --help               Print command line options.

Examples:
    $ kes key dek my-key
`

func dekCmd(args []string) {
	cmd := flag.NewFlagSet(args[0], flag.ContinueOnError)
	cmd.Usage = func() { tui.Fprint(os.Stderr, dekCmdUsage) }

	var (
		enclaveName string
	)
	cmd.StringVarP(&enclaveName, "enclave", "e", "", "Operate within the specified enclave")
	if err := cmd.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		cli.Fatalf("%v. See 'kes key dek --help'", err)
	}

	switch {
	case cmd.NArg() == 0:
		cli.Fatal("no key name specified. See 'kes key dek --help'")
	case cmd.NArg() > 2:
		cli.Fatal("too many arguments. See 'kes key dek --help'")
	}

	var associatedData []byte
	name := cmd.Arg(0)
	if cmd.NArg() == 2 {
		b, err := base64.StdEncoding.DecodeString(cmd.Arg(1))
		if err != nil {
			cli.Fatalf("invalid context: %v. See 'kes key dek --help'", err)
		}
		associatedData = b
	}

	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancelCtx()

	client := newClient(config{})
	key, err := client.GenerateKey(ctx, name, associatedData)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(1)
		}
		cli.Fatalf("failed to derive key: %v", err)
	}

	var (
		plaintext  = base64.StdEncoding.EncodeToString(key.Plaintext)
		ciphertext = base64.StdEncoding.EncodeToString(key.Ciphertext)
	)
	if cli.IsTerminal() {
		const format = "\nplaintext:  %s\nciphertext: %s\n"
		tui.Printf(format, plaintext, ciphertext)
	} else {
		const format = `{"plaintext":"%s","ciphertext":"%s"}`
		tui.Printf(format, plaintext, ciphertext)
	}
}
