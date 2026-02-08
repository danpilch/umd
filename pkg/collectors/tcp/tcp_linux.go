//go:build linux

package tcp

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers TCP/IP stack USE metrics on Linux.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	// Utilization: retransmit rate from /proc/net/snmp
	retransRate, err := getRetransmitRate()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Utilization,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "/proc/net/snmp",
		})
	} else {
		status := use.StatusOK
		if retransRate > 1.0 {
			status = use.StatusWarning
		}
		if retransRate > 5.0 {
			status = use.StatusError
		}
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Utilization,
			Value:       fmt.Sprintf("%.2f%% retrans", retransRate),
			RawValue:    retransRate,
			Status:      status,
			Description: "TCP retransmit rate (RetransSegs/OutSegs)",
			Command:     "/proc/net/snmp",
		})
	}

	// Saturation: listen queue overflows from /proc/net/netstat
	overflows, err := getListenOverflows()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Saturation,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "/proc/net/netstat",
		})
	} else {
		status := use.StatusOK
		if overflows > 0 {
			status = use.StatusWarning
		}
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Saturation,
			Value:       fmt.Sprintf("%d overflows", overflows),
			RawValue:    float64(overflows),
			Status:      status,
			Description: "Listen queue overflows + drops",
			Command:     "/proc/net/netstat",
		})
	}

	// Errors: TIME_WAIT count from /proc/net/tcp
	timeWait, err := getTimeWaitCount()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Errors,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "/proc/net/tcp",
		})
	} else {
		status := use.StatusOK
		if timeWait > 1000 {
			status = use.StatusWarning
		}
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Errors,
			Value:       fmt.Sprintf("%d TIME_WAIT", timeWait),
			RawValue:    float64(timeWait),
			Status:      status,
			Description: "Connections in TIME_WAIT state",
			Command:     "/proc/net/tcp",
		})
	}

	return checks, nil
}

func getRetransmitRate() (float64, error) {
	file, err := os.Open("/proc/net/snmp")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var tcpHeaders []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Tcp:") {
			if tcpHeaders == nil {
				tcpHeaders = strings.Fields(line)
			} else {
				// This is the values line
				values := strings.Fields(line)
				outSegsIdx := -1
				retransIdx := -1
				for i, h := range tcpHeaders {
					if h == "OutSegs" {
						outSegsIdx = i
					}
					if h == "RetransSegs" {
						retransIdx = i
					}
				}
				if outSegsIdx >= 0 && retransIdx >= 0 && outSegsIdx < len(values) && retransIdx < len(values) {
					outSegs, _ := strconv.ParseFloat(values[outSegsIdx], 64)
					retrans, _ := strconv.ParseFloat(values[retransIdx], 64)
					if outSegs > 0 {
						return (retrans / outSegs) * 100, nil
					}
				}
				return 0, nil
			}
		}
	}
	return 0, fmt.Errorf("TCP stats not found in /proc/net/snmp")
}

func getListenOverflows() (int64, error) {
	file, err := os.Open("/proc/net/netstat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var tcpExtHeaders []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "TcpExt:") {
			if tcpExtHeaders == nil {
				tcpExtHeaders = strings.Fields(line)
			} else {
				values := strings.Fields(line)
				var overflows, drops int64
				for i, h := range tcpExtHeaders {
					if i < len(values) {
						if h == "ListenOverflows" {
							overflows, _ = strconv.ParseInt(values[i], 10, 64)
						}
						if h == "ListenDrops" {
							drops, _ = strconv.ParseInt(values[i], 10, 64)
						}
					}
				}
				return overflows + drops, nil
			}
		}
	}
	return 0, fmt.Errorf("TcpExt not found in /proc/net/netstat")
}

func getTimeWaitCount() (int64, error) {
	file, err := os.Open("/proc/net/tcp")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var count int64
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue // skip header
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		// State is field 3 (0-indexed), TIME_WAIT = 06
		state := fields[3]
		if state == "06" {
			count++
		}
	}
	return count, nil
}

