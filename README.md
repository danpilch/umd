# umd - USE Method Daemon

Netflix-style deep performance engineering tool implementing Brendan Gregg's USE Method for system health analysis. Zero external runtime dependencies beyond Go stdlib and `golang.org/x/sys`.

## Quick Start

```bash
go build ./cmd/umd/
./umd              # Run all 8 collectors
./umd --score      # Include health score
./umd -f json      # JSON output
./umd -r cpu       # Check specific resource
./umd -w           # Continuous monitoring with sparklines
```

## Resource Collectors

umd checks 8 system resources across Utilization, Saturation, and Errors:

| Resource | Utilization | Saturation | Errors |
|----------|-------------|------------|--------|
| **CPU** | Busy % (sampling) | Load average / CPU count | Kernel log errors |
| **Memory** | Used % | Swap usage / pageouts | OOM / jetsam events |
| **Disk** | I/O busy % / throughput | Queue depth / TPS | I/O errors |
| **Network** | Throughput (bytes/s) | Dropped packets | Interface errors |
| **Scheduler** | Run queue depth | Context switches/sec | Involuntary CSW |
| **TCP** | Retransmit rate | Listen queue overflows | TIME_WAIT count |
| **VMem** | Major page fault rate | Swap I/O + page scan rate | Dirty page ratio |
| **Filesystem** | Inode usage % | FD utilization % | Zero free inodes |

## Output Formats

```bash
./umd -f table  # Styled terminal table (default)
./umd -f json   # Machine-readable JSON
./umd -f ai     # LLM-friendly markdown with drill-down suggestions
./umd -f tsv    # Tab-separated values for scripting
```

## Subcommands

### Workload Characterization

Answer "what is the system actually doing?"

```bash
./umd workload              # Top CPU/memory consumers, process states, load trend
./umd workload -n 20        # Top 20 processes
./umd workload -f json      # JSON output
```

### Flame Graph Capture

CPU profiling with SVG flame graph generation (requires elevated privileges):

```bash
./umd flamegraph                    # 10s system-wide capture
./umd flamegraph -d 30 -F 99       # 30s at 99Hz
./umd flamegraph -p 1234           # Profile specific PID
./umd flamegraph -o profile.svg    # Custom output path
```

Uses `perf` on Linux, `dtrace`/`sample` on macOS. Pure Go SVG renderer -- no external dependencies for graph generation.

### Performance Baselines

Save snapshots and detect drift:

```bash
./umd baseline save --name before-deploy   # Save current state
./umd baseline list                        # List saved baselines
./umd baseline compare --name before-deploy # Compare current vs saved
```

Baselines stored as JSON in `~/.umd/baselines/`.

### Self-Benchmarking

Validate the tool isn't perturbing what it measures:

```bash
./umd benchmark           # 20 iterations per collector
./umd benchmark -n 50     # 50 iterations
```

Reports P50/P95/P99 latency per collector, value stability (stddev), and tool memory overhead.

## Debug & Validation Flags

```bash
./umd --crosscheck    # Cross-validate metrics from multiple sources
./umd --trace         # Collector timing report to stderr
./umd --raw           # Raw metric dump to stderr
./umd --pprof         # Start Go pprof server on :6060
./umd --score         # Health score (0-100: Healthy/Degraded/Critical)
```

### Cross-Check Validation

`--crosscheck` reads the same metric from multiple OS sources and flags discrepancies:
- CPU: `host_processor_info` vs `top` (macOS), `/proc/stat` vs `sysinfo` (Linux)
- Memory: `host_statistics64` vs `vm_stat` (macOS), `/proc/meminfo` vs `sysinfo` (Linux)

Status: **VALID** (<5% deviation), **SUSPECT** (5-20%), **CONFLICT** (>20%)

## Watch Mode

Continuous monitoring with sparkline trend indicators:

```bash
./umd -w                    # Refresh every 2s
./umd -w -i 5 --score       # Every 5s with health score
```

The TREND column shows Unicode sparkline history for each metric.

## Thresholds

```bash
./umd --warn-util 80 --crit-util 95   # Custom utilization thresholds
```

Default: Warning at 70%, Critical at 90%.

## Architecture

```
cmd/umd/           CLI entry point + subcommands
pkg/use/            Core types (Check, Collector, Thresholds, Checker)
pkg/collectors/     8 resource collectors (cpu, memory, disk, network,
                    scheduler, tcp, vmem, filesystem)
pkg/output/         Formatters (table, json, ai, tsv), sparklines,
                    health scoring, drill-down suggestions
pkg/crosscheck/     Cross-validation engine + alternative metric sources
pkg/debug/          pprof server, timing decorator, trace logger, raw dump
pkg/flamegraph/     CPU capture + stack collapsing + SVG renderer
pkg/workload/       Process analysis + load characterization
pkg/baseline/       Baseline save/load + drift detection
pkg/benchmark/      Self-benchmarking engine
```

All collectors implement the `use.Collector` interface. Platform-specific code in `_linux.go` and `_darwin.go` files. Linux has full features; macOS degrades gracefully where data sources are limited.

## Platform Support

- **Linux**: Full support via `/proc`, `/sys`, `sysinfo`, `perf`
- **macOS**: Full support via Mach APIs, `sysctl`, `vm_stat`, `iostat`, `netstat`, `dtrace`

## Dependencies

- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/sirupsen/logrus` - Structured logging
- `github.com/spf13/cobra` - CLI framework
- `golang.org/x/sys` - System calls
