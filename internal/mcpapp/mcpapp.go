package mcpapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PeterStoica/chatwire/internal/cert"
	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/daemon"
	"github.com/PeterStoica/chatwire/internal/dial"
	"github.com/PeterStoica/chatwire/internal/linker"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/linkpage"
	"github.com/PeterStoica/chatwire/internal/mcptools"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/qrpage"
	"github.com/PeterStoica/chatwire/internal/shim"
	"github.com/PeterStoica/chatwire/internal/store"
)

const (
	versionTimeout = 5 * time.Second
	defaultLinger  = 10 * time.Minute
	reopenAfter    = 2 * time.Minute
	parentCheck    = 5 * time.Second
	buildIDLength  = 16
)

func Main() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "chatwire:", strings.TrimPrefix(err.Error(), "chatwire: "))
		return 1
	}
	return 0
}

func defaultState() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "chatwire", "linked.json")
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if exe, err := os.Executable(); err == nil && runtime.GOOS == "windows" {
		_ = os.Remove(exe + ".old")
	}
	command := "stdio"
	if len(args) > 0 && (!strings.HasPrefix(args[0], "-") || slices.Contains([]string{"-h", "-help", "--help"}, args[0])) {
		command, args = args[0], args[1:]
	}
	if command == "stdio" && len(args) == 0 && terminal(stdin) {
		command = "help"
	}
	switch command {
	case "help", "-h", "--help":
		_, err := io.WriteString(stdout, usage)
		return err
	case "version", "--version":
		_, err := fmt.Fprintln(stdout, "chatwire "+version())
		return err
	case "update":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(stderr)
		flags.Usage = usageOf(flags, stdout)
		check := flags.Bool("check", false, "only say whether a newer version exists")
		asJSON := flags.Bool("json", false, "print results as JSON")
		if err := flags.Parse(args); err != nil {
			return helped(err)
		}
		return runUpdate(ctx, stdout, *check, *asJSON)
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = usageOf(flags, stdout)
	state := flags.String("state", defaultState(), "where the linked device is kept (owner-only file)")
	linger := flags.Duration("linger", defaultLinger, "how long the shared background process stays up after the last window closes")
	asJSON := flags.Bool("json", false, "print results as JSON")
	phone := flags.String("phone", "", "link with a code for this mobile number instead of a QR code")
	wait := &seconds{d: -1}
	flags.Var(wait, "wait", "how long to wait for the result, in seconds (60) or as a duration (1m)")
	rest := args
	if command != "setup" {
		if err := flags.Parse(args); err != nil {
			return helped(err)
		}
		rest = flags.Args()
	}
	c := cli{ctx: ctx, state: *state, linger: *linger, out: stdout, asJSON: *asJSON, human: terminal(stdin)}
	switch command {
	case "stdio":
		return viaDaemon(ctx, *state, *linger, stdin, stdout, stderr)
	case "daemon":
		return serveDaemon(ctx, *state, *linger)
	case "serve":
		return serveAlone(ctx, *state, stdin, stdout)
	case "status":
		return c.status(max(wait.d, 0))
	case "link":
		return c.link(*phone, waitFor(wait.d, c.human))
	case "setup":
		return c.setupCommand(rest, stdin)
	}
	return fmt.Errorf("unknown command %q; run chatwire help", command)
}

func usageOf(flags *flag.FlagSet, out io.Writer) func() {
	return func() {
		_, _ = io.WriteString(out, usage)
		_, _ = fmt.Fprintf(out, "\nOptions for %s:\n", flags.Name())
		flags.SetOutput(out)
		flags.PrintDefaults()
	}
}

func helped(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

type seconds struct {
	d time.Duration
}

func (s *seconds) String() string {
	if s == nil || s.d < 0 {
		return ""
	}
	return s.d.String()
}

func (s *seconds) Set(value string) error {
	if n, err := strconv.ParseFloat(value, 64); err == nil && n >= 0 {
		s.d = time.Duration(n * float64(time.Second))
		return nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d < 0 {
		return fmt.Errorf("give seconds like 60 or a duration like 1m, not %q", value)
	}
	s.d = d
	return nil
}

func waitFor(asked time.Duration, human bool) time.Duration {
	switch {
	case asked >= 0:
		return asked
	case human:
		return defaultLinkFor
	}
	return 0
}

func launcher(state string, linger time.Duration) (shim.Launcher, error) {
	executable, build, err := self()
	if err != nil {
		return shim.Launcher{}, err
	}
	socket, err := socketPath(state)
	if err != nil {
		return shim.Launcher{}, err
	}
	return shim.Launcher{
		Socket: socket, Log: filepath.Join(filepath.Dir(state), "daemon.log"), Build: build, Executable: executable,
		DaemonArgs: []string{"daemon", "-state", state, "-linger", linger.String()}, Settings: ownSettings(),
	}, nil
}

func viaDaemon(ctx context.Context, state string, linger time.Duration, stdin io.Reader, stdout, stderr io.Writer) error {
	ctx, stop := shim.WhileParentLives(ctx, os.Getppid, parentCheck)
	defer stop()
	launcher, err := launcher(state, linger)
	if err != nil {
		return err
	}
	conn, err := launcher.Connect(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "chatwire: serving this window on its own, without the shared background process: %v\n", err)
		return serveAlone(ctx, state, stdin, stdout)
	}
	return shim.Pipe(ctx, conn, stdin, stdout)
}

func serveDaemon(ctx context.Context, state string, linger time.Duration) error {
	socket, err := socketPath(state)
	if err != nil {
		return err
	}
	listener, err := daemon.Listen(ctx, socket)
	if errors.Is(err, daemon.ErrRunning) {
		return nil
	}
	if err != nil {
		return err
	}
	_, build, err := self()
	if err != nil {
		_ = listener.Close()
		return err
	}
	a, err := open(ctx, state)
	if err != nil {
		_ = listener.Close()
		return err
	}
	defer a.close()
	return daemon.New(listener, build, linger, func(ctx context.Context, conn io.ReadWriteCloser, settings url.Values) {
		_ = a.server(settings).Run(ctx, &mcp.IOTransport{Reader: conn, Writer: conn})
	}).Serve(ctx)
}

func serveAlone(ctx context.Context, state string, stdin io.Reader, stdout io.Writer) error {
	a, err := open(ctx, state)
	if err != nil {
		return err
	}
	defer a.close()
	return a.server(ownSettings()).Run(ctx, &mcp.IOTransport{Reader: io.NopCloser(stdin), Writer: nopWriteCloser{stdout}})
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func socketPath(state string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("mcpapp: cache dir: %w", err)
	}
	absolute, err := filepath.Abs(filepath.Dir(state))
	if err != nil {
		return "", fmt.Errorf("mcpapp: state dir: %w", err)
	}
	sum := sha256.Sum256([]byte(absolute))
	name := hex.EncodeToString(sum[:4]) + ".sock"
	for _, base := range []string{cache, os.Getenv("XDG_RUNTIME_DIR"), privateTemp()} {
		if base == "" {
			continue
		}
		if path := filepath.Join(base, "chatwire", name); daemon.CheckPath(path) == nil {
			return path, nil
		}
	}
	return filepath.Join(cache, "chatwire", name), nil
}

func privateTemp() string {
	if runtime.GOOS == "linux" {
		return ""
	}
	return os.TempDir()
}

func self() (executable, build string, err error) {
	executable, err = os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("mcpapp: executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	executable = filepath.Clean(executable)
	f, err := os.Open(executable)
	if err != nil {
		return "", "", fmt.Errorf("mcpapp: build id: %w", err)
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", "", fmt.Errorf("mcpapp: build id: %w", err)
	}
	return executable, hex.EncodeToString(sum.Sum(nil))[:buildIDLength], nil
}

type app struct {
	lock     *os.File
	updates  *updates
	messages *store.Store
	m        *messenger.Messenger
	l        *linker.Linker
	media    string
	repeats  *mcptools.Repeats
	mu       sync.Mutex
	page     *linkpage.Page
	opened   time.Time
}

func (a *app) linkPage(ctx context.Context) (string, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.page == nil {
		page, err := linkpage.Start(ctx, a.l, rand.Reader)
		if err != nil {
			return "", false, err
		}
		a.page = page
	}
	if a.page.Open() || time.Since(a.opened) < reopenAfter {
		return a.page.URL, true, nil
	}
	if err := qrpage.Open(ctx, a.page.URL); err != nil {
		return a.page.URL, false, nil
	}
	a.opened = time.Now()
	return a.page.URL, true, nil
}

func open(ctx context.Context, path string) (*app, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mcpapp: state dir: %w", err)
	}
	lock, err := acquire(ctx, filepath.Dir(path), lockWait)
	if err != nil {
		return nil, err
	}
	a, err := openLocked(ctx, path)
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	a.lock, a.updates = lock, newUpdates(filepath.Dir(path))
	go a.updates.watch(ctx)
	return a, nil
}

func openLocked(ctx context.Context, path string) (*app, error) {
	browser := dial.Client(http.DefaultTransport)
	versions := dial.NewVersions(browser, dial.Page, versionTimeout)
	go versions.Current()
	dictionary, err := node.LoadDictionary()
	if err != nil {
		return nil, err
	}
	state, linked, err := load(path)
	if err != nil {
		return nil, err
	}
	cfg := linkflow.Config{
		Dial:       dial.Dialer(browser),
		Dictionary: dictionary,
		Root:       cert.WhatsAppRoot(),
		Version:    versions.Current,
		Random:     rand.Reader,
		Now:        time.Now,
		Save:       func(linked linkflow.Linked) error { return save(path, client.State{Linked: linked}) },
	}
	messages, err := store.Open(ctx, filepath.Join(filepath.Dir(path), "messages.db"))
	if err != nil {
		return nil, err
	}
	m := messenger.New(cfg, browser, func(s client.State) error { return save(path, s) }, messages)
	m.WhenOutdated(versions.Refresh)
	var existing *pairing.Account
	if linked {
		existing = &state.Linked.Account
	}
	l := linker.New(cfg, existing)
	l.WhenLinked(func(linked linkflow.Linked) { m.Start(ctx, client.State{Linked: linked}) })
	m.WhenLoggedOut(func(err error) {
		_ = os.Remove(path)
		l.LoggedOut(err)
	})
	if linked {
		m.Start(ctx, state)
	}
	a := &app{messages: messages, m: m, l: l, media: filepath.Join(filepath.Dir(path), "media"), repeats: mcptools.NewRepeats()}
	l.KeepQRWhile(a.pageSeen)
	return a, nil
}

func (a *app) pageSeen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.page != nil && a.page.Seen()
}

func (a *app) server(settings url.Values) *mcp.Server {
	return mcptools.NewServer(&mcp.Implementation{Name: "chatwire", Version: version()}, a.l, a.m, mcptools.Options{
		MediaDir: a.media, Folders: folders(settings.Get(settingFiles)), Private: []string{filepath.Dir(a.media)}, LinkPage: a.linkPage,
		Update: a.updates.available, Repeats: a.repeats, ReadOnly: settings.Get(settingReadOnly) == "1",
	})
}

func (a *app) close() {
	a.mu.Lock()
	if a.page != nil {
		_ = a.page.Close()
	}
	a.mu.Unlock()
	a.l.Close()
	a.m.Close()
	_ = a.messages.Close()
	_ = a.lock.Close()
}

func load(path string) (client.State, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return client.State{}, false, nil
	}
	if err != nil {
		return client.State{}, false, fmt.Errorf("mcpapp: read %s: %w", path, err)
	}
	var state client.State
	if err := json.Unmarshal(raw, &state); err != nil {
		return client.State{}, false, fmt.Errorf("mcpapp: decode %s: %w", path, err)
	}
	return state, true, nil
}

func save(path string, state client.State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mcpapp: state dir: %w", err)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("mcpapp: encode: %w", err)
	}
	if err := os.WriteFile(path+".tmp", encoded, 0o600); err != nil {
		return fmt.Errorf("mcpapp: write: %w", err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return fmt.Errorf("mcpapp: replace: %w", err)
	}
	return nil
}

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

const (
	settingFiles     = "files"
	settingReadOnly  = "read_only"
	readOnlyVariable = "CHATWIRE_READ_ONLY"
)

func ownSettings() url.Values {
	settings := url.Values{}
	if files := os.Getenv(mcptools.FilesVariable); files != "" {
		settings.Set(settingFiles, files)
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(readOnlyVariable))) {
	case "1", "true", "yes", "on":
		settings.Set(settingReadOnly, "1")
	}
	return settings
}

func folders(allowed string) []string {
	out := mcptools.DefaultFolders()
	for _, extra := range filepath.SplitList(allowed) {
		if extra = strings.TrimSpace(extra); filepath.IsAbs(extra) {
			out = append(out, extra)
		}
	}
	return out
}
