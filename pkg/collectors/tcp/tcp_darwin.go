//go:build darwin

package tcp

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/danpilch/umd/pkg/use"
)

// Collect gathers TCP/IP stack USE metrics on macOS.
func (c *Collector) Collect(thresholds use.Thresholds) ([]use.Check, error) {
	checks := make([]use.Check, 0, 3)

	// Utilization: retransmit info from netstat -s
	retransRate, err := getRetransmitRate()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Utilization,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "netstat -s",
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
			Description: "TCP retransmit rate",
			Command:     "netstat -s",
		})
	}

	// Saturation: listen queue overflows from netstat -s
	overflows, err := getListenOverflows()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Saturation,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "netstat -s",
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
			Description: "Listen queue overflows",
			Command:     "netstat -s",
		})
	}

	// Errors: connection states from netstat -an
	timeWait, err := getTimeWaitCount()
	if err != nil {
		checks = append(checks, use.Check{
			Resource:    "TCP",
			Type:        use.Errors,
			Value:       "unknown",
			Status:      use.StatusUnknown,
			Description: err.Error(),
			Command:     "netstat -an",
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
			Command:     "netstat -an",
		})
	}

	return checks, nil
}

func getRetransmitRate() (float64, error) {
	cmd := exec.Command("netstat", "-s", "-p", "tcp")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var segsSent, retrans float64
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "data packets") && strings.Contains(line, "bytes)") {
			// "12345 data packets (6789012 bytes)"
			fields := strings.Fields(line)
			if len(fields) > 0 {
				val, err := strconv.ParseFloat(fields[0], 64)
				if err == nil {
					segsSent = val
				}
			}
		}
		if strings.Contains(line, "data packet") && strings.Contains(line, "retransmit") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				val, err := strconv.ParseFloat(fields[0], 64)
				if err == nil {
					retrans = val
				}
			}
		}
	}

	if segsSent > 0 {
		return (retrans / segsSent) * 100, nil
	}
	return 0, nil
}

func getListenOverflows() (int64, error) {
	cmd := exec.Command("netstat", "-s", "-p", "tcp")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var overflows int64
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "listen queue overflow") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				val, _ := strconv.ParseInt(fields[0], 10, 64)
				overflows += val
			}
		}
	}
	return overflows, nil
}

func getTimeWaitCount() (int64, error) {
	cmd := exec.Command("netstat", "-an", "-p", "tcp")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var count int64
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "TIME_WAIT") {
			count++
		}
	}
	return count, nil
}
