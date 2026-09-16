#!/usr/bin/env bash
# perf.sh - hold how long a world takes to make to its baseline.
#
#   scripts/perf.sh check      run the benchmarks and fail if any is slower than
#                              the newest baseline by more than PERF_THRESHOLD
#                              percent with benchstat calling it significant
#   scripts/perf.sh baseline   run the benchmarks and write a new dated baseline
#   scripts/perf.sh compare A B
#                              benchstat two files already run
#   scripts/perf.sh scaling    make globes at 128, 256 and 512 wide, PERF_COUNT (3)
#                              times each, and fail if a tile at 512 costs more
#                              than PERF_SCALING times a tile at 256
#
# Settings, from the environment:
#   PERF_BENCH      benchmark regex   (default: the worlds that take seconds,
#                                      not the full globe)
#   PERF_COUNT      runs per world    (default 6: benchstat's least for a
#                                      confidence interval)
#   PERF_THRESHOLD  percent slower that fails check (default 10)
#   PERF_NEW        check this file, already run, instead of running now
#   PERF_SCALING    ns/tile at 512 over ns/tile at 256 that fails scaling
#                   (default 1.3: an n log n pass costs 1.13 per doubling of
#                   width, a quadratic one 4)
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
scaling="${PERF_SCALING:-1.3}"
scount="${PERF_COUNT:-3}"
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
scaling)
	# One world per run (-benchtime 1x), PERF_COUNT runs per width, and the
	# median ns/tile of each width read against the next. This catches a
	# pass whose cost per tile grows with the map - a superlinear step - and
	# not a slower constant, which moves every width alike and is check's
	# to catch. The median stands up to one run that the machine interrupted.
	if [[ -n "${PERF_NEW:-}" ]]; then
		new="$PERF_NEW"
	else
		new="$(mktemp -t terra-perf-scaling.XXXXXX)"
		echo "perf: go test -bench 'NewLand/globe(128|256|512)$' -benchtime 1x -count $scount > $new" >&2
		go test -run '^$' -bench 'NewLand/globe(128|256|512)$' -benchmem -benchtime 1x -count "$scount" -timeout 120m . >"$new"
	fi
	awk -v limit="$scaling" '
		/^BenchmarkNewLand\/globe(128|256|512)-/ {
			name = $1; sub(/^BenchmarkNewLand\//, "", name); sub(/-[0-9]+$/, "", name)
			for (i = 2; i < NF; i++) if ($(i + 1) == "ns/tile") { v[name, ++n[name]] = $i + 0 }
		}
		function median(name,    k, m, i, j, t, a) {
			m = n[name]
			for (i = 1; i <= m; i++) a[i] = v[name, i]
			for (i = 2; i <= m; i++) for (j = i; j > 1 && a[j-1] > a[j]; j--) { t = a[j]; a[j] = a[j-1]; a[j-1] = t }
			if (m % 2) return a[(m + 1) / 2]
			return (a[m / 2] + a[m / 2 + 1]) / 2
		}
		END {
			split("globe128 globe256 globe512", w, " ")
			for (i = 1; i <= 3; i++) {
				if (!n[w[i]]) { printf "perf: no ns/tile for %s in the output\n", w[i]; exit 2 }
				med[w[i]] = median(w[i])
				printf "perf: %-9s %8.1f ns/tile  (median of %d: ", w[i], med[w[i]], n[w[i]]
				for (k = 1; k <= n[w[i]]; k++) printf "%s%.1f", (k > 1 ? ", " : ""), v[w[i], k]
				printf ")\n"
			}
			r = med["globe512"] / med["globe256"]
			printf "perf: 512/256 = %.3f (limit %s); 256/128 = %.3f\n", r, limit, med["globe256"] / med["globe128"]
			if (r > limit + 0) { printf "perf: FAILED - a tile costs %.0f%% more at 512 than at 256: a pass is growing faster than the map\n", 100 * (r - 1); exit 1 }
			print "perf: ok"
		}
	' "$new"
	;;
*)
	sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
esac
