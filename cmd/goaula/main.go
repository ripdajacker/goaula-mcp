// Command goaula is a scripting-friendly CLI for Aula: everything the
// goaula-mcp MCP server is capable of, as one binary with subcommands.
//
// Human-readable output goes to stdout by default; pass --json (before or
// after the subcommand) for machine-readable JSON. Errors go to stderr with
// non-zero exit codes: 2 for usage errors, 1 for failures (including an
// invalid session, reported with an actionable message).
//
// # Session
//
// The session token lives in ~/.aula/session.txt, same as the client library
// and the MCP server. `goaula login` creates it interactively via MitID;
// `goaula session status` checks it and `goaula session ping --watch`
// keep-alives it (cron-friendly).
//
// # Context
//
// Many data tools are scoped to the current child/institution, just like the
// MCP server's aula_set_context. `goaula set-context` persists the selection
// to ~/.aula/context.json; every data command reads it, and --child/--institution
// override it per invocation.
//
// # Data commands
//
//	whoami           profile, children, institutions (with the IDs)
//	people           search people by name and role
//	posts            recent posts
//	messages         searched messages
//	threads          message threads
//	thread           a full thread by ID
//	contacts         contact list of a group
//	contact-groups   the groups each child belongs to
//	calendar         calendar events
//	daily-overview   daily presence overview
//	gallery          gallery albums with media URLs
//	gallery download download gallery media to files
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	aula "github.com/ripdajacker/goaula-mcp"
)

const (
	exitOK    = 0
	exitFail  = 1
	exitUsage = 2
)

func main() {
	// Data commands host this CLI in scripts: keep the INFO log lines (e.g.
	// "API usage verified") out of stderr unless debugging is requested.
	level := slog.LevelWarn
	if os.Getenv("GOAULA_DEBUG") != "" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	args := popGlobalFlags(os.Args[1:])
	if len(args) == 0 {
		usage(os.Stderr)
		os.Exit(exitUsage)
	}

	var exit int
	switch args[0] {
	case "help", "--help", "-h":
		usage(os.Stdout)
	case "login":
		exit = cmdLogin(args[1:])
	case "session":
		exit = cmdSession(args[1:])
	case "whoami":
		exit = cmdWhoami(args[1:])
	case "children":
		exit = cmdChildren(args[1:])
	case "set-context":
		exit = cmdSetContext(args[1:])
	case "people":
		exit = cmdPeople(args[1:])
	case "posts":
		exit = cmdPosts(args[1:])
	case "messages":
		exit = cmdMessages(args[1:])
	case "threads":
		exit = cmdThreads(args[1:])
	case "thread":
		exit = cmdThread(args[1:])
	case "contacts":
		exit = cmdContacts(args[1:])
	case "contact-groups":
		exit = cmdContactGroups(args[1:])
	case "calendar":
		exit = cmdCalendar(args[1:])
	case "daily-overview":
		exit = cmdDailyOverview(args[1:])
	case "gallery":
		exit = cmdGallery(args[1:])
	case "gallery-download":
		exit = cmdGalleryDownload(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "goaula: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		exit = exitUsage
	}
	os.Exit(exit)
}

// usage prints the command overview.
func usage(w *os.File) {
	fmt.Fprint(w, `goaula — CLI for Aula (aula.dk)

Usage: goaula [--json] <command> [flags]

Session:
  login              Interactive MitID login (creates ~/.aula/session.txt)
  session status     Check whether the session is valid (exit 0/1)
  session ping       One ping; --watch keep-alives with an interval

Data (all accept --json; --child/--institution override the context):
  whoami             Profile, children, institutions (with IDs)
  children           The children of the profile (IDs for --child flags)
  set-context        Persist the child/institution context for future commands
  people             Search people by name and role
  posts              Recent posts
  messages           Search messages
  threads            List message threads
  thread             One thread with all messages
  contact-groups     Groups (classes, SFO) each child belongs to
  contacts           Contact list of a group
  calendar           Calendar events
  daily-overview     Daily presence overview
  gallery            Gallery albums with media URLs
  gallery-download   Download gallery media to files

Get started: goaula login && goaula whoami
Use "goaula <command> -h" for command-specific flags.
`)
}

// popGlobalFlags removes "--json"/"-json" tokens preceding the subcommand and
// re-appends "--json" at the end, where every command's flag set accepts it as
// the global output switch (so both `goaula --json posts` and
// `goaula posts --json` work).
func popGlobalFlags(args []string) []string {
	n := 0
	jsonSeen := false
	for n < len(args) && (args[n] == "--json" || args[n] == "-json") {
		jsonSeen = true
		n++
	}
	if n == 0 {
		return args
	}
	rest := args[n:] // the subcommand and its arguments (possibly empty)
	if jsonSeen {
		rest = append(rest, "--json")
	}
	return rest
}

// flagSet creates a flag set with the common flags of every command: -json
// (global output switch) and the per-command child/institution overrides.
// The register callback adds the command's own flags BEFORE parsing.
func flagSet(args []string, name string, register func(fs *flag.FlagSet)) (*flag.FlagSet, *bool, context) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	ctx := context{}
	fs.Int64Var(&ctx.ChildID, "child", 0, "child ID for the command context (overrides the persisted context); 0 = persisted/default")
	fs.Int64Var(&ctx.InstitutionID, "institution", 0, "institution profile ID for the command context (overrides the persisted context); 0 = persisted/default")
	if register != nil {
		register(fs)
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}
	return fs, jsonOut, ctx
}

// requireArgs verifies the positional argument count after the flags.
func requireArgs(fs *flag.FlagSet, want int) {
	if fs.NArg() != want {
		fmt.Fprintf(os.Stderr, "goaula: %q takes %d argument(s), got %d\n", fs.Name(), want, fs.NArg())
		fs.Usage()
		os.Exit(exitUsage)
	}
}

// context is the effective command context: which child/institution profile to
// scope the client to. Zero values mean "no explicit selection".
type context struct {
	ChildID       int64 `json:"child"`
	InstitutionID int64 `json:"institution"`
}

// persistedContext is the ~/.aula/context.json file format.
type persistedContext = context

func contextFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.FromSlash(".aula/context.json")
	}
	return filepath.Join(home, ".aula", "context.json")
}

// loadContext reads the persisted context file, returning the zero context
// when it is missing or unreadable (the first set-context run matters more
// than failing every command).
func loadContext() persistedContext {
	data, err := os.ReadFile(contextFile())
	if err != nil {
		return persistedContext{}
	}
	var ctx persistedContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		fmt.Fprintf(os.Stderr, "goaula: ignoring unreadable context file %s: %v\n", contextFile(), err)
		return persistedContext{}
	}
	return ctx
}

// saveContext writes the context file (atomically-ish: direct write is fine
// for a 2-int JSON).
func saveContext(ctx persistedContext) error {
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(contextFile()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(contextFile(), data, 0o600)
}

// sessionClient builds a client with the session file provider, performs the
// login (the validation of the session token) and applies the context: the
// explicit flag overrides, then the persisted file selection.
func sessionClient(ctx context) (*aula.Client, error) {
	client := aula.NewClient(&aula.Config{Session: aula.NewFileSessionProvider()})
	if err := client.Login(); err != nil {
		return nil, err
	}
	applyContext(client, mergeContext(loadContext(), ctx))
	return client, nil
}

// mergeContext layers the command flags over the persisted context file.
func mergeContext(persisted, flags context) context {
	merged := persisted
	if flags.ChildID != 0 {
		merged.ChildID = flags.ChildID
	}
	if flags.InstitutionID != 0 {
		merged.InstitutionID = flags.InstitutionID
	}
	return merged
}

// applyContext selects the child/institution on the client, best-effort: the
// data methods fall back to "all children" when the selection is missing.
func applyContext(client *aula.Client, ctx context) {
	if ctx.ChildID != 0 && !client.SetMyCurrentChild(ctx.ChildID) {
		fmt.Fprintf(os.Stderr, "goaula: warning: child %d not found; using the profile's defaults\n", ctx.ChildID)
	}
	if ctx.InstitutionID != 0 && !client.SetMyCurrentInstitution(ctx.InstitutionID) {
		fmt.Fprintf(os.Stderr, "goaula: warning: institution profile %d not found; using the profile's defaults\n", ctx.InstitutionID)
	}
}

// jsonOutPrint writes a JSON line to stdout.
func jsonOutPrint(result any) error {
	data, err := aula.MarshalJSON(result)
	if err != nil {
		return err
	}
	os.Stdout.Write(data)
	fmt.Println()
	return nil
}

// emit renders the result: JSON via --json, otherwise human text.
func emit(jsonOut bool, result any, human func(*strings.Builder)) {
	if jsonOut {
		data, err := aula.MarshalJSON(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "goaula: failed to serialize result: %v\n", err)
			os.Exit(exitFail)
		}
		os.Stdout.Write(data)
		fmt.Println()
		return
	}
	var b strings.Builder
	human(&b)
	fmt.Print(b.String())
}

// failSession reports a session/login failure with an actionable message.
func failSession(err error) {
	denied := new(aula.AccessDeniedError)
	if errors.As(err, &denied) {
		fmt.Fprintf(os.Stderr, "goaula: %v\n", err)
		fmt.Fprintln(os.Stderr, "       Retry the command; if it keeps failing, run 'goaula login'")
		fmt.Fprintln(os.Stderr, "       and complete the secure-login step-up for elevated methods.")
		return
	}
	invalid := new(aula.InvalidSessionError)
	if errors.As(err, &invalid) {
		fmt.Fprintf(os.Stderr, "goaula: %v\n", err)
		fmt.Fprintln(os.Stderr, "       Run 'goaula login', or refresh ~/.aula/session.txt (goaula-login), and retry.")
		return
	}
	fmt.Fprintf(os.Stderr, "goaula: %v\n", err)
}
