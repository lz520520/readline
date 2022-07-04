package readline

import (
	"bufio"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type Terminal struct {
	m         sync.Mutex
	cfg       *Config
	outchan   chan rune
	closed    int32
	stopChan  chan struct{}
	kickChan  chan struct{}
	wg        sync.WaitGroup
	isReading int32
	sleeping  int32

	writeMutex MultiMuTex

	sizeChan chan string
}

func NewTerminal(cfg *Config) (*Terminal, error) {
	if err := cfg.Init(); err != nil {
		return nil, err
	}
	t := &Terminal{
		cfg:      cfg,
		kickChan: make(chan struct{}, 1),
		outchan:  make(chan rune),
		stopChan: make(chan struct{}, 1),
		sizeChan: make(chan string, 1),
	}

	go t.ioloop()
	return t, nil
}

// SleepToResume will sleep myself, and return only if I'm resumed.
func (t *Terminal) SleepToResume() {
	if !atomic.CompareAndSwapInt32(&t.sleeping, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&t.sleeping, 0)

	t.ExitRawMode()
	ch := WaitForResume()
	SuspendMe()
	<-ch
	t.EnterRawMode()
}

func (t *Terminal) EnterRawMode() (err error) {
	return t.cfg.FuncMakeRaw()
}

func (t *Terminal) ExitRawMode() (err error) {
	return t.cfg.FuncExitRaw()
}

func (t *Terminal) Write(b []byte) (int, error) {
	needUnlock := false
	// 检查是否上锁，无则继续打印，有的话，说明有地方要求prompt打印比其他优先
	// 这时，会判断调用堆栈，如果是Readline+print，说明是prompt打印，并且不能直接解锁，需要等打印完解锁
	if t.writeMutex.CheckLockStatus() {
		buf := make([]byte, 4096)
		runtime.Stack(buf, false)
		if strings.Contains(string(buf), "readline.(*Instance).Readline(") &&
			strings.Contains(string(buf), "readline.(*RuneBuffer).print(") {
			needUnlock = true
		}
	}
	n, err := t.cfg.Stdout.Write(b)

	if needUnlock {
		t.writeMutex.Unlock()
	}
	return n, err
}

// WriteStdin prefill the next Stdin fetch
// Next time you call ReadLine() this value will be writen before the user input
func (t *Terminal) WriteStdin(b []byte) (int, error) {
	return t.cfg.StdinWriter.Write(b)
}

type termSize struct {
	left int
	top  int
}

func (t *Terminal) GetOffset(f func(offset string)) {
	go func() {
		f(<-t.sizeChan)
	}()
	t.Write([]byte("\033[6n"))
}

func (t *Terminal) WriteLock() {
	t.writeMutex.Lock()
}
func (t *Terminal) IsWriteLock() bool {
	return t.writeMutex.CheckLockStatus()
}

func (t *Terminal) WriteWait() {
	t.writeMutex.Wait()
}
func (t *Terminal) WriteUnLock() {
	t.writeMutex.Unlock()
}

func (t *Terminal) Print(s string) {
	fmt.Fprintf(t.cfg.Stdout, "%s", s)
}

func (t *Terminal) PrintRune(r rune) {
	fmt.Fprintf(t.cfg.Stdout, "%c", r)
}

func (t *Terminal) Readline() *Operation {
	return NewOperation(t, t.cfg)
}

// return rune(0) if meet EOF
func (t *Terminal) ReadRune() rune {
	ch, ok := <-t.outchan
	if !ok {
		return rune(0)
	}
	return ch
}

func (t *Terminal) IsReading() bool {
	return atomic.LoadInt32(&t.isReading) == 1
}

func (t *Terminal) KickRead() {
	select {
	case t.kickChan <- struct{}{}:
	default:
	}
}

func (t *Terminal) ioloop() {
	t.wg.Add(1)
	defer func() {
		t.wg.Done()
		close(t.outchan)
	}()

	var (
		isEscape       bool
		isEscapeEx     bool
		expectNextChar bool
	)

	buf := bufio.NewReader(t.getStdin())
	for {
		if !expectNextChar {
			atomic.StoreInt32(&t.isReading, 0)
			select {
			case <-t.kickChan:
				atomic.StoreInt32(&t.isReading, 1)
			case <-t.stopChan:
				return
			}
		}
		expectNextChar = false
		r, _, err := buf.ReadRune()
		if err != nil {
			if strings.Contains(err.Error(), "interrupted system call") {
				expectNextChar = true
				continue
			}
			break
		}

		if isEscape {
			isEscape = false
			if r == CharEscapeEx {
				expectNextChar = true
				isEscapeEx = true
				continue
			}
			r = escapeKey(r, buf)
		} else if isEscapeEx {
			isEscapeEx = false
			if key := readEscKey(r, buf); key != nil {
				r = escapeExKey(key)
				// offset
				if key.typ == 'R' {
					if _, _, ok := key.Get2(); ok {
						select {
						case t.sizeChan <- key.attr:
						default:
						}
					}
					expectNextChar = true
					continue
				}
			}
			if r == 0 {
				expectNextChar = true
				continue
			}
		}

		expectNextChar = true
		switch r {
		case CharEsc:
			if t.cfg.VimMode {
				t.outchan <- r
				break
			}
			isEscape = true
		case CharInterrupt, CharEnter, CharCtrlJ, CharDelete:
			expectNextChar = false
			fallthrough
		default:
			t.outchan <- r
		}
	}

}

func (t *Terminal) Bell() {
	fmt.Fprintf(t, "%c", CharBell)
}

func (t *Terminal) Close() error {
	if atomic.SwapInt32(&t.closed, 1) != 0 {
		return nil
	}
	if closer, ok := t.cfg.Stdin.(io.Closer); ok {
		closer.Close()
	}
	close(t.stopChan)
	t.wg.Wait()
	return t.ExitRawMode()
}

func (t *Terminal) GetConfig() *Config {
	t.m.Lock()
	cfg := *t.cfg
	t.m.Unlock()
	return &cfg
}

func (t *Terminal) getStdin() io.Reader {
	t.m.Lock()
	r := t.cfg.Stdin
	t.m.Unlock()
	return r
}

func (t *Terminal) SetConfig(c *Config) error {
	if err := c.Init(); err != nil {
		return err
	}
	t.m.Lock()
	t.cfg = c
	t.m.Unlock()
	return nil
}
