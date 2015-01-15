package cmdctrl_test

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"testing"

	"github.com/facebookgo/cmdctrl"
)

func listen(start chan struct{}, fin chan struct{}, count int) {
	defer close(fin)
	if count == 0 {
		close(start)
		return
	}

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGUSR2, syscall.SIGKILL)
	close(start)
	for {
		sig := <-ch
		switch sig {
		case syscall.SIGTERM:
			fmt.Println("SIGTERM")
		case syscall.SIGUSR2:
			fmt.Println("SIGUSR2")
		case syscall.SIGKILL:
			fmt.Println("SIGKILL")
		}
		count--
		if count == 0 {
			return
		}
	}
}

func getCount() int {
	countStr := os.Getenv("COUNT")
	if countStr == "" {
		return 0
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		fmt.Printf("error converting COUNT %s to int: %s\n", countStr, err)
		os.Exit(42)
	}
	return count
}

func testBin() {
	start := make(chan struct{})
	fin := make(chan struct{})
	go listen(start, fin, getCount())
	<-start

	cmdctrl.SimpleStart()
	<-fin
	fmt.Println("successfully started")
}

const (
	testBinMode      = "TEST_BIN_MODE"
	testBinModeValue = "42"
)

func TestMain(m *testing.M) {
	if os.Getenv(testBinMode) == testBinModeValue {
		testBin()
		return
	}
	os.Exit(m.Run())
}
