package system

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"

	"github.com/aelsabbahy/GOnetstat"
	// This needs a better name
	util2 "github.com/aelsabbahy/goss/util"
	"github.com/codegangsta/cli"
	"github.com/coreos/go-systemd/dbus"
	"github.com/coreos/go-systemd/util"
	"github.com/mitchellh/go-ps"
)

type Resource interface {
	Exists() (interface{}, error)
}

type System struct {
	NewPackage  func(string, *System, util2.Config) Package
	NewFile     func(string, *System, util2.Config) File
	NewAddr     func(string, *System, util2.Config) Addr
	NewPort     func(string, *System, util2.Config) Port
	NewService  func(string, *System, util2.Config) Service
	NewUser     func(string, *System, util2.Config) User
	NewGroup    func(string, *System, util2.Config) Group
	NewCommand  func(string, *System, util2.Config) Command
	NewDNS      func(string, *System, util2.Config) DNS
	NewProcess  func(string, *System, util2.Config) Process
	NewGossfile func(string, *System, util2.Config) Gossfile
	dbus        *dbus.Conn
	ports       map[string][]GOnetstat.Process
	dbusOnce    sync.Once
	portsOnce   sync.Once
	procOnce    sync.Once
	procMap     map[string][]ps.Process
}

func (s *System) Ports() map[string][]GOnetstat.Process {
	s.portsOnce.Do(func() {
		s.ports = GetPorts(false)
	})
	return s.ports
}

func (s *System) Dbus() *dbus.Conn {
	s.dbusOnce.Do(func() {
		dbus, err := dbus.New()
		if err != nil {
			fmt.Println(err)
			// FIXME: Do we really want to exit here?
			os.Exit(1)
		}
		s.dbus = dbus
	})
	return s.dbus
}

func (s *System) ProcMap() map[string][]ps.Process {
	s.procOnce.Do(func() {
		s.procMap = GetProcs()
	})
	return s.procMap
}

func New(c *cli.Context) *System {
	sys := &System{
		NewFile:     NewDefFile,
		NewAddr:     NewDefAddr,
		NewPort:     NewDefPort,
		NewUser:     NewDefUser,
		NewGroup:    NewDefGroup,
		NewCommand:  NewDefCommand,
		NewDNS:      NewDefDNS,
		NewProcess:  NewDefProcess,
		NewGossfile: NewDefGossfile,
	}
	// FIXME: Detect-os needs to be refactored in a consistent way
	// Also, cache should be its own object
	sys.detectService()

	switch {
	case c.GlobalString("package") == "rpm":
		sys.NewPackage = NewRpmPackage
	case c.GlobalString("package") == "deb":
		sys.NewPackage = NewDebPackage
	default:
		sys.NewPackage = detectPackage()
	}

	return sys
}

func detectPackage() func(string, *System, util2.Config) Package {
	switch {
	case isRpm():
		return NewRpmPackage
	case isDeb():
		return NewDebPackage
	default:
		return NewNullPackage
	}
}

func (s *System) detectService() {
	switch {
	case util.IsRunningSystemd():
		s.NewService = NewServiceDbus
	case isUbuntu():
		s.NewService = NewServiceUpstart
	default:
		s.NewService = NewServiceInit
	}
}

func isUbuntu() bool {
	if b, err := ioutil.ReadFile("/etc/lsb-release"); err == nil {
		if bytes.Contains(b, []byte("Ubuntu")) {
			return true
		}
	}
	return false

}
func isDeb() bool {
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return true
	}

	// See if it has only one of the package managers
	if hasCommand("dpkg") && !hasCommand("rpm") {
		return true
	}

	return false
}

func isRpm() bool {
	if _, err := os.Stat("/etc/redhat-release"); err == nil {
		return true
	}

	if _, err := os.Stat("/etc/system-release"); err == nil {
		return true
	}

	// See if it has only one of the package managers
	if hasCommand("rpm") && !hasCommand("dpkg") {
		return true
	}
	return false
}

func hasCommand(cmd string) bool {
	if _, err := exec.LookPath(cmd); err == nil {
		return true
	}
	return false
}
