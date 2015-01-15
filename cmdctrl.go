// Package cmdctrl provides standard startup logic for servers.
package cmdctrl

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/facebookgo/devrestarter"
	"github.com/facebookgo/flagconfig"
	"github.com/facebookgo/flagenv"
	"github.com/facebookgo/pidfile"
	"github.com/facebookgo/stdfd"
)

// Provides ProcessControl for the current process.
var CurrentProcessControl = ProcessControl(os.Getpid())

// Provides control over the process identified by the underlying pid.
type ProcessControl int

// Send a signal to the process to gracefully stop.
func (p ProcessControl) Stop() error {
	return p.sendSignal(syscall.SIGTERM)
}

// Send a signal to the process to gracefully restart.
func (p ProcessControl) Restart() error {
	return p.sendSignal(syscall.SIGUSR2)
}

// Send a signal to forcefully kill the server dropping all active connections.
func (p ProcessControl) Kill() error {
	return p.sendSignal(syscall.SIGKILL)
}

func (p ProcessControl) sendSignal(sig os.Signal) error {
	process, err := os.FindProcess(int(p))
	if err != nil {
		return err
	}

	if err := process.Signal(sig); err != nil {
		return err
	}

	return nil
}

func usage() {
	flagenv.Parse()
	flagconfig.Parse()
	fmt.Fprintf(
		os.Stderr,
		"usage: %s [options...] {start|stop|kill|restart}\n",
		filepath.Base(os.Args[0]),
	)
	flag.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(os.Stderr, "  -%s=%s: %s\n", f.Name, f.Value.String(), f.Usage)
	})
}

func prestart(enableDevRestarter bool, stdout, stderr string) error {
	if enableDevRestarter {
		devrestarter.Init()
	}
	if err := stdfd.RedirectOutputs(stdout, stderr); err != nil {
		return err
	}
	if err := pidfile.Write(); err != nil {
		return err
	}
	return nil
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(2)
	}
}

func mustPidfilePC() ProcessControl {
	pid, err := pidfile.Read()
	exitOnError(err)
	return ProcessControl(pid)
}

// Simple Start is a os.Exiting version of the standard startup procedure. This
// provides simpler semantics and serves as an easy to use API for the standard
// case by automatically handling flag parsing, stop/restart/kill commands and
// only returning when the requested command was a "start" command.
func SimpleStart() {
	stdout := flag.String("stdout", "", "file path to redirect stdout to")
	stderr := flag.String("stderr", "", "file path to redirect stderr to")
	goMaxProcs := flag.Int("gomaxprocs", runtime.NumCPU(), "gomaxprocs")
	enableDevRestarter := flag.Bool(
		"devrestarter", false, "if true the devrestarter will be enabled")

	flag.Usage = usage
	flag.Parse()
	flagenv.Parse()
	flagconfig.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "no command was specified")
		flag.Usage()
		os.Exit(1)
	}

	runtime.GOMAXPROCS(*goMaxProcs)

	cmd := flag.Arg(0)
	switch cmd {
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		flag.Usage()
		os.Exit(1)
	case "start":
		exitOnError(prestart(*enableDevRestarter, *stdout, *stderr))
		return
	case "stop":
		exitOnError(mustPidfilePC().Stop())
	case "kill":
		exitOnError(mustPidfilePC().Kill())
	case "restart":
		pid, err := pidfile.Read()
		if err == nil {
			err = ProcessControl(pid).Restart()
			if err == nil {
				break
			}
		}
		fmt.Printf("%s restart error: %s. trying fresh start.\n", filepath.Base(os.Args[0]), err)
		exitOnError(prestart(*enableDevRestarter, *stdout, *stderr))
		return
	}

	os.Exit(0)
}
