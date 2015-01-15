package cmdctrl_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var binPath string

func init() {
	binPath, _ = exec.LookPath(os.Args[0])
}

func binCmd(arg ...string) *exec.Cmd {
	cmd := exec.Command(binPath, arg...)
	cmd.Env = append(
		[]string{fmt.Sprintf("%s=%s", testBinMode, testBinModeValue)},
		os.Environ()...,
	)
	return cmd
}

func tempfilepath(t *testing.T, suffix string) string {
	file, err := ioutil.TempFile("", "cmdctrltest."+suffix)
	if err != nil {
		t.Fatal(err)
	}

	err = file.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = os.Remove(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	return file.Name()
}

// Waits until the file exists.
func waitFile(p string) {
	for {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}
}

func TestNoCommand(t *testing.T) {
	t.Parallel()
	out, err := binCmd().CombinedOutput()
	if err == nil {
		t.Fatalf("was expecting error but got: %s", out)
	}
}

func TestFreshStart(t *testing.T) {
	t.Parallel()
	p := tempfilepath(t, "pid")
	defer os.Remove(p)
	out, err := binCmd("-pidfile", p, "start").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	successfullyStarted := []byte("successfully started\n")
	if !bytes.Equal(out, successfullyStarted) {
		t.Fatalf(
			"expected output to contain %s but got\n%s",
			successfullyStarted,
			out,
		)
	}
}

func TestRestart(t *testing.T) {
	t.Parallel()
	p := tempfilepath(t, "pid")
	defer os.Remove(p)
	started := make(chan bool)
	finished := make(chan bool)
	go func() {
		defer func() { finished <- true }()
		cmd := binCmd("-pidfile", p, "start")
		cmd.Env = append(cmd.Env, "COUNT=1")
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b

		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		waitFile(p)
		started <- true

		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}

		successfullyRestarted := []byte("SIGUSR2\nsuccessfully started\n")
		if !bytes.Equal(b.Bytes(), successfullyRestarted) {
			t.Fatalf(
				`expected output to contain "%s" but got "%s"`,
				successfullyRestarted,
				b.Bytes(),
			)
		}
	}()

	<-started
	cmd := binCmd("-pidfile", p, "restart")
	restartOut, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	if len(restartOut) != 0 {
		t.Fatalf("expected no restart output but got: %s", restartOut)
	}

	<-finished
}

func TestRestartWhenNotRunning(t *testing.T) {
	t.Parallel()
	p := tempfilepath(t, "pid")
	defer os.Remove(p)
	out, err := binCmd("-pidfile", p, "restart").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	freshStartOnRestart := "%s restart error: open %s: no such file or " +
		"directory. trying fresh start.\nsuccessfully started\n"
	expected := []byte(fmt.Sprintf(freshStartOnRestart, filepath.Base(binPath), p))
	if !bytes.Equal(out, expected) {
		t.Fatalf("expected output to contain %s but got\n%s", expected, out)
	}
}

func TestStop(t *testing.T) {
	t.Parallel()
	p := tempfilepath(t, "pid")
	defer os.Remove(p)
	started := make(chan bool)
	finished := make(chan bool)
	go func() {
		defer func() { finished <- true }()
		cmd := binCmd("-pidfile", p, "start")
		cmd.Env = append(cmd.Env, "COUNT=1")
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b

		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		waitFile(p)
		started <- true

		if err := cmd.Wait(); err != nil {
			t.Fatal(err, b.String())
		}

		successfullyStopped := []byte("SIGTERM\nsuccessfully started\n")
		if !bytes.Equal(b.Bytes(), successfullyStopped) {
			t.Fatalf(
				`expected output to contain "%s" but got "%s"`,
				successfullyStopped,
				b.Bytes(),
			)
		}
	}()

	<-started
	stopOut, err := binCmd("-pidfile", p, "stop").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	if len(stopOut) != 0 {
		t.Fatalf("expected no stop output but got: %s", stopOut)
	}

	<-finished
}

func TestStopWhenNotRunning(t *testing.T) {
	t.Parallel()
	p := tempfilepath(t, "pid")
	defer os.Remove(p)
	out, err := binCmd("-pidfile", p, "stop").CombinedOutput()
	if err == nil {
		t.Fatalf("was expecting error but got %s", out)
	}
	expected := []byte(fmt.Sprintf("open %s: no such file or directory", p))
	if !bytes.Equal(out, expected) {
		t.Fatalf("expected output to contain %s but got\n%s", expected, out)
	}
}

func TestKill(t *testing.T) {
	t.Parallel()
	p := tempfilepath(t, "pid")
	defer os.Remove(p)
	started := make(chan bool)
	finished := make(chan bool)
	go func() {
		defer func() { finished <- true }()
		cmd := binCmd("-pidfile", p, "start")
		cmd.Env = append(cmd.Env, "COUNT=1")
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b

		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		waitFile(p)
		started <- true

		if err := cmd.Wait(); err == nil {
			t.Fatalf("expected error but got output: %s", b.Bytes())
		}

		if b.Len() != 0 {
			t.Fatalf(`expected empty output but got "%s"`, b.Bytes())
		}
	}()

	<-started
	killOut, err := binCmd("-pidfile", p, "kill").CombinedOutput()
	if err != nil {
		t.Fatalf(`got error running kill: "%s" with output: "%s"`, err, killOut)
	}

	if len(killOut) != 0 {
		t.Fatalf("expected no kill output but got: %s", killOut)
	}

	<-finished
}
