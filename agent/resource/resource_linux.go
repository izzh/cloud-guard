package resource

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func init() {
	if s, err := os.ReadFile("/sys/class/dmi/id/product_serial"); err == nil {
		hostSerial = string(bytes.TrimSpace(s))
	}
	if s, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		hostID = string(bytes.TrimSpace(s))
	}
	if s, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		hostModel = string(bytes.TrimSpace(s))
	}
	if s, err := os.ReadFile("/sys/class/dmi/id/sys_vendor"); err == nil {
		hostVendor = string(bytes.TrimSpace(s))
	}
}
func GetDNS() string {
	var svrs []string
	if f, err := os.Open("/etc/resolv.conf"); err == nil {
		s := bufio.NewScanner(f)
		for s.Scan() {
			if strings.HasPrefix(s.Text(), "nameserver") {
				svrs = append(svrs, strings.TrimSpace(strings.TrimPrefix(s.Text(), "nameserver")))
			}
		}
		f.Close()
	}
	return strings.Join(svrs, ",")
}
func GetGateway() string {
	if f, err := os.Open("/proc/net/route"); err == nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			fields := strings.Fields(s.Text())
			if len(fields) > 3 {
				if flags, err := strconv.ParseInt(fields[3], 16, 64); err == nil {
					if (flags&unix.RTF_UP == unix.RTF_UP) && (flags&unix.RTF_GATEWAY == unix.RTF_GATEWAY) {
						if ipa, err := hex.DecodeString(fields[2]); err == nil && len(ipa) == 4 {
							return net.IP{ipa[3], ipa[2], ipa[1], ipa[0]}.String()
						}
					}
				}
			}
		}
	}
	return ""
}
func GetTCPCount() int {
	var count int
	for _, tcpFile := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		tcpData, err := os.ReadFile(tcpFile)
		if err == nil {
			tcps := bytes.Split(tcpData, []byte("\n"))
			count = len(tcps)
			if count > 1 {
				count--
			}
		}
	}
	return count
}

type ReceiveBytes uint64
type TransmitBytes uint64

func UploadDownloadFlow(dev string) (DownloadFlow, UploadFlow, error) {
	down, up, err := TotalFlowByDevice(dev)
	if err != nil {
		return "", "", err
	}
	time.Sleep(time.Second * 1)

	down2, up2, err := TotalFlowByDevice(dev)
	if err != nil {
		return "", "", err
	}

	downStr := int64(down2 - down)
	upStr := int64(up2 - up)

	return DownloadFlow(downStr), UploadFlow(upStr), nil
}

func TotalFlowByDevice(dev string) (ReceiveBytes, TransmitBytes, error) {
	devInfo, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}

	var receive int = -1
	var transmit int = -1

	var receiveBytes uint64
	var transmitBytes uint64

	lines := strings.Split(string(devInfo), "\n")
	for _, line := range lines {
		if strings.Contains(line, dev) {
			i := 0
			fields := strings.Split(line, ":")
			for _, field := range fields {
				if strings.Contains(field, dev) {
					i = 1
				} else {
					values := strings.Fields(field)
					for _, value := range values {
						//logger.Debug(value)
						if receive == i {
							bytes, _ := strconv.ParseInt(value, 10, 64)
							receiveBytes = uint64(bytes)
						} else if transmit == i {
							bytes, _ := strconv.ParseInt(value, 10, 64)
							transmitBytes = uint64(bytes)
						}
						i++
					}
				}
			}
		} else if strings.Contains(line, "face") {
			index := 0
			tag := false
			fields := strings.Split(line, "|")
			for _, field := range fields {
				if strings.Contains(field, "face") {
					index = 1
				} else if strings.Contains(field, "bytes") {
					values := strings.Fields(field)
					for _, value := range values {
						//logger.Debug(value)
						if strings.Contains(value, "bytes") {
							if !tag {
								tag = true
								receive = index
							} else {
								transmit = index
							}
						}
						index++
					}
				}
			}
		}
	}

	return ReceiveBytes(receiveBytes), TransmitBytes(transmitBytes), nil
}
