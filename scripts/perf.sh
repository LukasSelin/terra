#!/usr/bin/env bash
# perf.sh - hold how long a world takes to make to its baseline.
#
#   scripts/perf.sh check      run the benchmarks and fail if any is slower than
#                              the newest baseline by more than PERF_THRESHOLD
#                              percent with benchstat calling it significant
#   scripts/perf.sh baseline   run the benchmarks and write a new dated baseline
#   scripts/perf.sh compare A B
#                              benchstat two files already run
#
# Settings, from the environment:
#   PERF_BENCH      benchmark regex   (default: the worlds that take seconds,
#                                      not the full globe)
#   PERF_COUNT      runs per world    (default 6: benchstat's least for a
#                                      confidence interval)
#   PERF_THRESHOLD  percent slower that fails check (default 10)
#   PERF_NEW        check this file, already run, instead of running now
#
# Baselines are only comparable on the machine they were taken on: check
# refuses a baseline whose cpu line is not this machine's. Nothing else heavy
# should run meanwhile. See docs/perf/README.md.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

bench="${PERF_BENCH:-NewLand/(valley|ancient|globe256)$}"
count="${PERF_COUNT:-6}"
threshold="${PERF_THRESHOLD:-10}"
basedir="docs/perf/baseline"

benchstat() {
	if command -v benchstat >/dev/null 2>&1; then
		command benchstat "$@"
	else
		go run golang.org/x/perf/cmd/benchstat@latest "$@"
	fi
}

run() {
	echo "perf: go test -bench '$bench' -count $count > $1" >&2
	go test -run '^$' -bench "$bench" -benchmem -count "$count" -timeout 120m . >"$1"
}

newest_baseline() {
	ls "$basedir"/*-small.txt 2>/dev/null | sort | tail -1
}

cpu_of() {
	grep -m1 '^cpu:' "$1" | sed 's/[[:space:]]*$//'
}

case "${1:-}" in
check)
	base="$(newest_baseline)"
	if [[ -z "$base" ]]; then
		echo "perf: no baseline in $basedir; run scripts/perf.sh baseline" >&2
		exit 2
	fi
	if [[ -n "${PERF_NEW:-}" ]]; then
		new="$PERF_NEW"
	else
		new="$(mktemp -t terra-perf.XXXXXX)"
		run "$new"
	fi
	if [[ "$(cpu_of "$base")" != "$(cpu_of "$new")" ]]; then
		echo "perf: $base was taken on '$(cpu_of "$base")', this is '$(cpu_of "$new")'" >&2
		echo "perf: times are not comparable across machines; take a baseline here first" >&2
		exit 2
	fi
	benchstat "$base" "$new"
	echo
	# In benchstat's CSV the sec/op table's rows are name, old, CI, new, CI,
	# delta, p. The delta is "~" where the difference is not significant.
	benchstat -format csv "$base" "$new" 2>/dev/null | awk -F, -v limit="$threshold" -v base="$base" '
		$2 == "sec/op" { table = 1; next }
		table && $1 == "" { table = 0 }
		table && $1 != "geomean" && $1 != "" {
			delta = $6
			if (delta == "~") { printf "perf: %-24s no significant change\n", $1; next }
			d = delta; sub(/%/, "", d); d += 0
			if (d > limit) {
				printf "perf: %-24s %s SLOWER than %s (limit +%s%%, %s)\n", $1, delta, base, limit, $7
				failed = 1
			} else if (d < -limit) {
				printf "perf: %-24s %s faster - take a new baseline so the gain is kept\n", $1, delta
			} else {
				printf "perf: %-24s %s, within +-%s%%\n", $1, delta, limit
			}
		}
		END { exit failed }
	' && echo "perf: ok" || { echo "perf: FAILED - world creation has drifted slower" >&2; exit 1; }
	;;
baseline)
	out="$basedir/$(date +%Y-%m-%d-%H%M)-small.txt"
	run "$out"
	echo "perf: wrote $out; commit it with a work log entry saying why" >&2
	;;
compare)
	[[ $# -eq 3 ]] || { echo "usage: $0 compare old.txt new.txt" >&2; exit 2; }
	benchstat "$2" "$3"
	;;
*)
	sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
