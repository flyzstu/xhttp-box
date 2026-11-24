//go:build linux || android || darwin || ios

package libbox

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
	"github.com/sagernet/sing/common"
	"golang.org/x/sys/unix"
)

type nativeShellSession struct {
	shell *os.File
	cmd   *exec.Cmd
}

func nativeCredential(uid, gid int32, groups Int32Iterator) *syscall.SysProcAttr {
	if uid == 0 && gid == 0 {
		return nil
	}
	return &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    uint32(uid),
			Gid:    uint32(gid),
			Groups: common.Map(iteratorToArray[int32](groups), func(g int32) uint32 { return uint32(g) }),
		},
	}
}

func OpenNativeShellSession(
	shell, cwd string,
	args, environ StringIterator,
	term string,
	rows, cols, uid, gid int32,
	groups Int32Iterator,
) (ShellSession, error) {
	envArray := iteratorToArray[string](environ)
	if term != "" {
		envArray = append(envArray, "TERM="+term)
	}
	command := exec.Command(shell, iteratorToArray[string](args)...)
	command.Dir = cwd
	command.Env = envArray
	command.SysProcAttr = nativeCredential(uid, gid, groups)
	master, err := pty.StartWithAttrs(command, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}, nil)
	if err != nil {
		return nil, err
	}
	return &nativeShellSession{shell: master, cmd: command}, nil
}

func OpenNativePipeSession(
	shell, cwd string,
	args, environ StringIterator,
	uid, gid int32,
	groups Int32Iterator,
) (ShellSession, error) {
	envArray := iteratorToArray[string](environ)
	if len(envArray) == 0 {
		envArray = os.Environ()
	}
	rawFds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}
	parent := os.NewFile(uintptr(rawFds[0]), "shell")
	child := os.NewFile(uintptr(rawFds[1]), "shell")
	command := exec.Command(shell, iteratorToArray[string](args)...)
	command.Dir = cwd
	command.Env = envArray
	command.Stdin = child
	command.Stdout = child
	command.Stderr = child
	command.SysProcAttr = nativeCredential(uid, gid, groups)
	if err := command.Start(); err != nil {
		parent.Close()
		child.Close()
		return nil, err
	}
	child.Close()
	return &nativeShellSession{shell: parent, cmd: command}, nil
}

func (s *nativeShellSession) MasterFD() int32 {
	return int32(s.shell.Fd())
}

func (s *nativeShellSession) Resize(rows, cols int32) error {
	return pty.Setsize(s.shell, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (s *nativeShellSession) Signal(sig int32) error {
	return s.cmd.Process.Signal(syscall.Signal(sig))
}

func (s *nativeShellSession) WaitExit() (int32, error) {
	err := s.cmd.Wait()
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return int32(exitErr.ExitCode()), nil
	}
	return 0, err
}

func (s *nativeShellSession) Close() error {
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.shell.Close()
}
