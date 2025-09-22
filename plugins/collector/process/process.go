package process

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bytedance/Elkeid/plugins/collector/utils"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/tklauser/go-sysconf"
)

const (
	TraversalInterval = time.Millisecond * 250
)

var (
	hertz    = int64(100)
	btime, _ = host.BootTime()
	pageSize = int64(os.Getpagesize())
	rpns     = ""
)

func init() {
	if h, err := sysconf.Sysconf(sysconf.SC_CLK_TCK); err == nil {
		hertz = h
	}
	if stat, err := os.Stat(filepath.Join("/proc", "self", "ns", "pid")); err == nil {
		if st, ok := stat.Sys().(*syscall.Stat_t); ok {
			rpns = strconv.FormatUint(st.Ino, 10)
		}
	}
}

type Process struct {
	pid     string
	exe     string
	cmdline string
	comm    string
}
type ProcessStat struct {
	Comm      string `mapstructure:"comm"`
	State     string `mapstructure:"state"`
	Ppid      string `mapstructure:"ppid"`
	Pgid      string `mapstructure:"pgid"`
	Sid       string `mapstructure:"sid"`
	StartTime string `mapstructure:"start_time"`
}
// ProcessCPU 存储进程的CPU使用信息
type ProcessCPU struct {
	Utime      string `mapstructure:"utime"`      // 用户模式下消耗的CPU时间
	Stime      string `mapstructure:"stime"`      // 系统模式下消耗的CPU时间
	Cutime     string `mapstructure:"cutime"`     // 所有子进程在用户模式下消耗的CPU时间
	Cstime     string `mapstructure:"cstime"`     // 所有子进程在系统模式下消耗的CPU时间
	Priority   string `mapstructure:"priority"`   // 进程优先级
	Nice       string `mapstructure:"nice"`       // 进程的nice值
}

// ProcessMem 存储进程的内存使用信息
type ProcessMem struct {
	RSS        string `mapstructure:"rss"`        // 常驻内存集大小（单位：字节）
	RSSLim     string `mapstructure:"rsslim"`     // 进程的RSS限制
	Vsize      string `mapstructure:"vsize"`      // 虚拟内存大小
}
type ProcessStatus struct {
	Umask      string `mapstructure:"umask"`
	TracerPid  string `mapstructure:"tcpid"`
	Ruid       string `mapstructure:"ruid"`
	Euid       string `mapstructure:"euid"`
	Suid       string `mapstructure:"suid"`
	Fsuid      string `mapstructure:"fsuid"`
	Rgid       string `mapstructure:"rgid"`
	Egid       string `mapstructure:"egid"`
	Sgid       string `mapstructure:"sgid"`
	Fsgid      string `mapstructure:"fsgid"`
	Rusername  string `mapstructure:"rusername"`
	Eusername  string `mapstructure:"eusername"`
	Susername  string `mapstructure:"susername"`
	Fsusername string `mapstructure:"fsusername"`
	NsPid      string `mapstructure:"nspid"`
	NsPgid     string `mapstructure:"nspgid"`
	NsSid      string `mapstructure:"nssid"`
}
type ProcessNamespace struct {
	Diff   string `mapstructure:"dns"`
	Cgroup string `mapstructure:"cns"`
	Ipc    string `mapstructure:"ins"`
	Mnt    string `mapstructure:"mns"`
	Net    string `mapstructure:"nns"`
	Pid    string `mapstructure:"pns"`
	Time   string `mapstructure:"tns"`
	User   string `mapstructure:"uns"`
	Uts    string `mapstructure:"utns"`
}

func PnsDiffWithRpns(pns string) bool { return pns != rpns }
func Processes(wk bool) (procs []Process, err error) {
	var entries []fs.DirEntry
	entries, err = os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range entries {
		if _, err := strconv.ParseInt(e.Name(), 10, 32); err == nil {
			if wk {
				procs = append(procs, Process{pid: e.Name()})
			} else {
				p := Process{pid: e.Name()}
				if _, err := p.Exe(); err == nil {
					procs = append(procs, p)
				}
			}
		}
	}
	return
}
func NewProcess(pid string) (p *Process, err error) {
	_, err = os.Stat(filepath.Join("/proc", pid))
	if err != nil {
		return
	}
	p = &Process{pid: pid}
	return
}
func (p *Process) Pid() string {
	return p.pid
}
func (p *Process) Stat() (s ProcessStat, err error) {
	var stat []byte
	stat, err = os.ReadFile(filepath.Join("/proc", p.pid, "stat"))
	if err != nil {
		return
	}
	fields := bytes.Fields(stat)
	if len(fields) > 24 {
		s.Comm = string(bytes.TrimFunc(fields[1], func(r rune) bool {
			return r == '(' || r == ')'
		}))
		s.State = string(fields[2])
		s.Ppid = string(fields[3])
		s.Pgid = string(fields[4])
		s.Sid = string(fields[5])
		if starttime, err := strconv.ParseInt(string(fields[21]), 10, 64); err == nil {
			s.StartTime = strconv.FormatInt(starttime/hertz+int64(btime), 10)
		}
	}
	return
}
// CPU 返回进程的CPU使用信息
func (p *Process) CPU() (c ProcessCPU, err error) {
	var stat []byte
	stat, err = os.ReadFile(filepath.Join("/proc", p.pid, "stat"))
	if err != nil {
		return
	}
	fields := bytes.Fields(stat)
	if len(fields) > 17 {
		// 获取CPU时间相关字段
		c.Utime = string(fields[13])  // 第14个字段：用户模式下的CPU时间
		c.Stime = string(fields[14])  // 第15个字段：系统模式下的CPU时间
		c.Cutime = string(fields[15]) // 第16个字段：子进程用户模式下的CPU时间
		c.Cstime = string(fields[16]) // 第17个字段：子进程系统模式下的CPU时间
		c.Priority = string(fields[17]) // 第18个字段：优先级
		c.Nice = string(fields[18])   // 第19个字段：nice值
	}
	return
}
// TotalCPUTime 计算进程占用的总CPU时间（单位：秒）
// includeChildren: 是否包含子进程的CPU时间
func (p *Process) TotalCPUTime(includeChildren bool) (totalTime string, err error) {
	cpuInfo, err := p.CPU()
	if err != nil {
		return "", err
	}

	// 解析用户时间和系统时间
	utime, err := strconv.ParseInt(cpuInfo.Utime, 10, 64)
	if err != nil {
		return "", err
	}
	stime, err := strconv.ParseInt(cpuInfo.Stime, 10, 64)
	if err != nil {
		return "", err
	}

	// 计算总CPU时间（时钟滴答数）
	totalTicks := utime + stime

	// 如果需要包含子进程的CPU时间
	if includeChildren {
		cutime, err := strconv.ParseInt(cpuInfo.Cutime, 10, 64)
		if err != nil {
			return "", err
		}
		cstime, err := strconv.ParseInt(cpuInfo.Cstime, 10, 64)
		if err != nil {
			return "", err
		}
		totalTicks += cutime + cstime
	}

	// 转换为秒（时钟滴答数 / 每秒的时钟滴答数）并格式化为字符串
	timeInSeconds := float64(totalTicks) / float64(hertz)
	totalTime = strconv.FormatFloat(timeInSeconds, 'f', 6, 64)
	return totalTime, nil
}

// Mem 返回进程的内存使用信息
func (p *Process) Mem() (m ProcessMem, err error) {
	var stat []byte
	stat, err = os.ReadFile(filepath.Join("/proc", p.pid, "stat"))
	if err != nil {
		return
	}
	fields := bytes.Fields(stat)
	if len(fields) > 23 {
		// 获取RSS值（单位：页），转换为字节
		if rssPages, err := strconv.ParseInt(string(fields[22]), 10, 64); err == nil {
			m.RSS = strconv.FormatInt(rssPages*pageSize, 10)
		}
		// 获取虚拟内存大小
		m.Vsize = string(fields[20])
		// 获取RSS限制
		m.RSSLim = string(fields[21])
	}
	return
}
func (p *Process) Status() (s ProcessStatus, err error) {
	var status []byte
	status, err = os.ReadFile(filepath.Join("/proc", p.pid, "status"))
	if err != nil {
		return
	}
	lines := bytes.FieldsFunc(status, func(r rune) bool { return r == '\n' })
	for _, line := range lines {
		fields := bytes.FieldsFunc(line, func(r rune) bool {
			return r == '\t'
		})
		if len(fields) < 2 {
			continue
		}
		key := string(fields[0])
		switch key {
		case "Umask:":
			s.Umask = string(fields[1])
		case "TracerPid:":
			s.TracerPid = string(fields[1])
		case "Uid:":
			if len(fields) < 5 {
				continue
			} else {
				s.Ruid = string(fields[1])
				s.Rusername, _ = utils.GetUsername(s.Ruid)
				s.Euid = string(fields[2])
				s.Eusername, _ = utils.GetUsername(s.Euid)
				s.Suid = string(fields[3])
				s.Susername, _ = utils.GetUsername(s.Suid)
				s.Fsuid = string(fields[4])
				s.Fsusername, _ = utils.GetUsername(s.Fsuid)
			}
		case "Gid:":
			if len(fields) < 5 {
				continue
			} else {
				s.Rgid = string(fields[1])
				s.Rusername, _ = utils.GetGroupname(s.Rgid)
				s.Egid = string(fields[2])
				s.Eusername, _ = utils.GetGroupname(s.Egid)
				s.Sgid = string(fields[3])
				s.Susername, _ = utils.GetGroupname(s.Sgid)
				s.Fsgid = string(fields[4])
				s.Fsusername, _ = utils.GetGroupname(s.Fsgid)
			}
		case "NSpid:":
			s.NsPid = string(fields[1])
		case "NSpgid:":
			s.NsPgid = string(fields[1])
		case "NSsid:":
			s.NsSid = string(fields[1])
		}
	}
	return
}
func (p *Process) Cmdline() (ret string, err error) {
	if p.cmdline != "" {
		ret = p.cmdline
		return
	}
	var cmdline []byte
	cmdline, err = os.ReadFile(filepath.Join("/proc", p.pid, "cmdline"))
	if err != nil {
		return
	}
	ret = string(bytes.TrimSpace(bytes.ReplaceAll(cmdline, []byte{0}, []byte{' '})))
	if len(ret) > 1024 {
		ret = ret[:1024]
	}
	p.cmdline = ret
	return
}
func (p *Process) Exe() (ret string, err error) {
	if p.exe != "" {
		return p.exe, nil
	}
	ret, err = os.Readlink(filepath.Join("/proc", p.pid, "exe"))
	ret = strings.TrimSpace(ret)
	p.exe = ret
	return
}
func (p *Process) ExeHash() (ret string, err error) {
	var exe string
	exe, err = p.Exe()
	if err != nil {
		return
	}
	return utils.GetHash(exe, filepath.Join("/proc", p.pid, "exe"))
}
func (p *Process) ExeChecksum() (ret string, err error) {
	var exe string
	exe, err = p.Exe()
	if err != nil {
		return
	}
	return utils.GetMd5(exe, filepath.Join("/proc", p.pid, "exe"))
}
func (p *Process) Comm() (ret string, err error) {
	var d []byte
	d, err = os.ReadFile(filepath.Join("/proc", p.pid, "comm"))
	if err != nil {
		return
	}
	ret = string(bytes.TrimSpace(d))
	p.comm = ret
	return
}
func (p *Process) Cwd() (ret string, err error) {
	ret, err = os.Readlink(filepath.Join("/proc", p.pid, "cwd"))
	ret = strings.TrimSpace(ret)
	return
}
func (p *Process) Namespaces() (n ProcessNamespace, err error) {
	_, err = os.Stat(filepath.Join("/proc", p.pid, "ns"))
	if err != nil {
		return
	}
	for _, ns := range []string{"cgroup", "ipc", "mnt", "net", "pid", "user", "uts"} {
		if stat, er := os.Stat(filepath.Join("/proc", p.pid, "ns", ns)); er == nil {
			if st, ok := stat.Sys().(*syscall.Stat_t); ok {
				switch ns {
				case "cgroup":
					n.Cgroup = strconv.FormatUint(st.Ino, 10)
				case "ipc":
					n.Ipc = strconv.FormatUint(st.Ino, 10)
				case "mnt":
					n.Mnt = strconv.FormatUint(st.Ino, 10)
				case "net":
					n.Net = strconv.FormatUint(st.Ino, 10)
				case "pid":
					n.Pid = strconv.FormatUint(st.Ino, 10)
				case "user":
					n.User = strconv.FormatUint(st.Ino, 10)
				case "uts":
					n.Uts = strconv.FormatUint(st.Ino, 10)
				}
			} else {
				err = errors.New("invalid ns stat")
				break
			}
		} else {
			err = er
			break
		}
	}
	if n.Pid == rpns {
		n.Diff = "false"
	} else {
		n.Diff = "true"
	}
	return
}
func (p *Process) Namespace(n string) (ret string, err error) {
	switch n {
	case "cgroup", "ipc", "mnt", "net", "pid", "user", "uts":
		var stat fs.FileInfo
		stat, err = os.Stat(filepath.Join("/proc", p.pid, "ns", n))
		if err != nil {
			return
		}
		if st, ok := stat.Sys().(*syscall.Stat_t); ok {
			ret = strconv.FormatUint(st.Ino, 10)
		}
	default:
		err = errors.New("unknown namespace type: " + n)
	}
	return
}

func (p *Process) Fds() (ret []string, err error) {
	var f *os.File
	f, err = os.Open(filepath.Join("/proc", p.pid, "fd"))
	if err != nil {
		return
	}
	defer f.Close()
	var names []string
	names, err = f.Readdirnames(1024)
	if err != nil {
		return
	}
	for _, n := range names {
		res, err := os.Readlink(filepath.Join("/proc", p.pid, "fd", n))
		if err != nil {
			continue
		}
		ret = append(ret, strings.TrimSpace(res))
	}
	return
}
func (p *Process) Envs() (ret map[string]string, err error) {
	ret = make(map[string]string, 10)
	var f *os.File
	f, err = os.Open(filepath.Join("/proc", p.pid, "environ"))
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.Index(data, []byte{0}); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return
	})
	for s.Scan() {
		if fields := strings.Split(s.Text(), "="); len(fields) == 2 {
			ret[fields[0]] = fields[1]
		}
	}
	return
}
