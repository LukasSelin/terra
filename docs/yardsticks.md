# Yardsticks and their noise

The yardsticks (`TestRealNumbers` in `yardstick_test.go`, `TestTheRealWorld`
in `realism_test.go`, `realism_shape_test.go` and `realism_soil_test.go`)
hold a made world to figures measured on the earth. Most read something a
world settles on whatever its seed: the sea floor's depth against its age,
the climate by latitude, the soils' shares. Those are read once and held as
points, as they always were.

A handful read the river networks and the relief, and those are their
history's. A change anywhere upstream (the rain, the rock, how the plates
were cut) redraws every network on every seed, and the reading moves as far
as one globe's rivers differ from another's, whether or not the change made
the world more like the earth. Read pooled over three or eight globes, those
readings swapped between pass and fail on every world-moving branch of the
Earth-system stack (#67, #71, #73, #74, #75), and every PR explained the swap
as seed-level chaos. They could not tell a better world from a worse one
(#76).

So those readings are read over seeds and given an interval, and they gate
on the interval, not the point. The bands and their sources are unchanged.

## How a reading is read

`spread_test.go` has the machinery. A yardstick with `seeded` set reads one
of two ways:

- **Median of seeds** (the default). Each seed's worlds give a reading of
  their own, fitted on their own. The yardstick reads the median of those
  readings. Its interval is the k-th least to the k-th greatest seed reading,
  for the greatest k that holds the median of all seeds with at least 90%
  (the distribution-free interval for a median, Conover 1999 §3.2). For
  sixteen seeds that is the 5th to the 12th (92.3%), for eight the 2nd to the
  7th (93.0%). Three seeds are never more than 75% sure of their median: their
  interval is the least to the greatest. The interval asks nothing of how the
  readings scatter, so one globe whose reading is wild (concavity −0.28,
  wavelength 533 m) moves it no further than any other seed on that side of
  the median would.
- **Pooled, jackknife.** One fit over every seed's worlds together, as the
  yardstick read before, with a 95% interval: Student's t for n − 1 degrees of
  freedom times the jackknife standard error over the seeds (how far the fit
  moves with each seed left out). This is for a reading that is not the same
  reading off one globe. A fit to the handful of area bins one small globe's
  basin has is a worse fit (Flint's R², whose median of single-globe fits is
  0.49 against the pooled 0.93) or a much noisier one (Hack's exponent: the
  per-globe median's interval is 0.07 wide, the pooled fit's 0.04).

**The gating rule.** A gating yardstick fails only when its interval lies
wholly outside the band. A known gap closes only when its interval lies
wholly inside the band. A reading whose interval on main, at the seeds the
suite makes, is wider than its band cannot tell a world in the band from one
outside it: it is **advisory** (`advisory` set), read and logged with its
interval on every run, and never a failure. A run with `-v` logs every
seeded reading's interval and its seeds.

**Comparing two branches.** A branch is better or worse than main on a
reading when the two intervals do not overlap; then whichever median lies
nearer the band (0 inside it) is the better. Overlapping intervals are the
same within the noise.

## The readings

Read on main at 2c51bea, on 2026-10-07. "sd a seed" is how far one seed's
reading scatters from the next. "Before" is the reading as it was until
this change, pooled over the seeds it read then, with the jackknife's
standard error over those seeds. "Seeds for a tenth" is how many seeds would
hold the reading's standard error to a tenth of the band: (1.2533 sd / (band
/ 10))² for a median, n (se / (band / 10))² for a pooled fit.

| reading | band | source | how | seeds | sd a seed | main: reading [interval] | interval / band | before: pooled (seeds) ± se | seeds for a tenth | gates? |
|---|---|---|---|---|---|---|---|---|---|---|
| hypsometric integral, small globe | 0.32–0.60 | Strahler 1952 | median | 16 | 0.043 | 0.361 [0.332, 0.388] | 0.20 | 0.305 (3) ± 0.016 | 4 | gates |
| drainage area exceedance exponent, small globe | 0.39–0.46 | Rodriguez-Iturbe 1992; Rigon 1996 | median | 16 | 0.124 | 0.479 [0.422, 0.561] | 1.99 | 0.486 (16) ± 0.026 | 493 | advisory |
| discharge exceedance exponent, small globe | 0.40–0.46 | Rodriguez-Iturbe 1992; Rigon 1996 | median | 16 | 0.116 | 0.438 [0.383, 0.553] | 2.83 | 0.462 (16) ± 0.027 | 587 | advisory |
| Hack exponent, small globe | 0.54–0.60 | Hack 1957; Rigon 1996 | pooled | 16 | 0.070 | 0.578 [0.558, 0.598] | 0.66 | 0.571 (8) ± 0.007 | 39 | gates |
| Hack exponent, globe | 0.54–0.60 | Hack 1957; Rigon 1996 | median | 3 full globes | 0.009 | 0.587 [0.578, 0.596] | 0.29 | 0.578 (1) | 4 | gates |
| ridge-valley wavelength, small globe | 24–224 m | Perron, Dietrich & Kirchner 2008 | median | 16 (15 read) | 149 | 133 [133, 320] | 0.93 | 133 (3) ± 0 | 88 | gates |
| channel concavity, small globe | 0.35–0.60 | Flint 1974; Tucker & Whipple 2002; Whipple 2004 | median | 16 | 0.218 | 0.233 [0.071, 0.336] | 1.06 | 0.246 (8) ± 0.102 | 120 | advisory |
| Flint's law fit R², small globe | 0.85–1 | Flint 1974; Wobus 2006 | pooled | 16 | 0.317 | 0.925 [0.790, 1.061] | 1.81 | 0.884 (8) ± 0.112 | 288 | advisory |
| Hack exponent, 2x less 1x, small globe | −0.05–0.05 | Hack 1957; Rigon 1996 | pooled | 8 pairs | 0.046 | 0.014 [−0.027, 0.056] | 0.83 | 0.013 (4) ± 0.013 | 25 | gates |
| channel concavity, 2x less 1x, small globe | −0.1–0.1 | Wobus 2006; Perron & Royden 2013 | median | 8 pairs | 0.244 | 0.172 [−0.068, 0.431] | 2.49 | 0.057 (4) ± 0.159 | 234 | advisory |
| hypsometric integral, 2x less 1x, small globe | −0.05–0.05 | Strahler 1952 | median | 8 pairs | 0.041 | 0.019 [−0.008, 0.071] | 0.79 | 0.038 (4) ± 0.028 | 27 | gates |
| land relief intermittency C1, three globes | 0.08–0.18 | Gagnon, Lovejoy & Schertzer 2006 | median | 3 full globes | 0.020 | 0.067 [0.063, 0.099] | 0.36 | 0.075 (3) ± 0.011 | 7 | gates (known gap K on main) |

The full source of each band is in the yardstick's `source`, unchanged.

What the table says:

- **The hypsometric integral of the small globes** failed on main at 0.3055,
  pooled over globes 1-3. Over sixteen its median is 0.361, and the interval
  lies wholly inside the band. Globes 1 and 2 read 0.305 and 0.277; the
  failure was those two globes.
- **The exceedance exponents** read one basin a small globe, and a basin's
  exponent scatters by 0.12. Over sixteen globes the interval is two to
  three times the band, and some 500 globes would be needed to narrow it to a
  tenth of the band. Both are advisory. Both failed on main.
- **Channel concavity on the small globes** scatters by 0.22 a globe (−0.28
  to 0.58 over sixteen). Its interval is a shade wider than its band, so it
  is advisory; but on main the whole interval, 0.07–0.34, lies **under** the
  band. The seeds do say main's channels are less concave than real ones.
  This is the gap the yardstick's comment describes reopening.
- **Flint's R²** needs about 300 small globes for its pooled fit to be read
  to a tenth of the band. Advisory.
- **Hack's exponent**, pooled over sixteen small globes, has a jackknife
  standard error of 0.009 and gates. On the full globes it was read off globe
  1 alone. It is now the median of the three globes the shape yardsticks
  already make (0.578, 0.596, 0.587).
- **The ridge-valley wavelength** is the peak of a spectrum, and most globes
  put it in the same bin (133 m): eight of fifteen read 133 or 145 m, and a few
  read 320-533 m. The median and its order-statistic interval are not moved by
  how far those few read, where a standard deviation would be. The pooled
  fit's jackknife reads ±0 here: the jackknife does not hold for a peak's
  bin, and that is why the pooled fit is not used.
- **The resolution readings** compared four double globes with four single
  ones. Hack's difference scatters by 0.046 a pair, and at four pairs its
  interval was wider than its band. They now read eight (`resolutionSeeds`;
  see the cost below). The concavity's difference scatters by 0.24 a pair
  and stays advisory at any affordable count. The hypsometric integral's
  difference gates at eight. `mean land rain, 2x over 1x` reads the same
  eight pairs; it is still a point reading, and it fails on main as before
  (1.246 at four pairs).
- **C1** is read off the three full globes, one fit a globe. On main it is
  a known gap (K), and a gap closes only when its interval lies wholly inside
  the band.

## Whether the swaps are gone: main against #73 and #75

Run with these yardsticks merged into `origin/claude/ocean-flow-2d` (#73,
0b54aef) and `origin/claude/air-circulation` (#75, 795764e), both on main
2c51bea. As they read before, pooled over their old seeds:

| reading | band | main | #73 | #75 |
|---|---|---|---|---|
| hypsometric integral, small globe | 0.32–0.6 | 0.3055 **fail** | 0.3654 pass | 0.3185 **fail** |
| drainage area exceedance exponent, small globe | 0.39–0.46 | 0.4863 **fail** | 0.4008 pass | 0.4138 pass |
| discharge exceedance exponent, small globe | 0.4–0.46 | 0.4622 **fail** | 0.4336 pass | 0.4674 **fail** |
| Hack exponent, small globe | 0.54–0.6 | 0.5711 pass | 0.5919 pass | 0.5293 **fail** |
| Hack exponent, globe | 0.54–0.6 | 0.5783 pass | 0.5938 pass | 0.6011 **fail** |
| ridge-valley wavelength, small globe | 24–224 | 133.3 pass | 320 **fail** | 145.5 pass |
| channel concavity, small globe | 0.35–0.6 | 0.2455 **fail** | 0.2563 **fail** | 0.4093 pass |
| Flint's law fit R2, small globe | 0.85–1 | 0.8844 pass | 0.979 pass | 0.9002 pass |
| Hack exponent, 2x less 1x, small globe | −0.05–0.05 | 0.01317 pass | 0.02369 pass | 0.07325 **fail** |
| channel concavity, 2x less 1x, small globe | −0.1–0.1 | 0.05723 pass | 0.06593 pass | −0.1318 **fail** |
| hypsometric integral, 2x less 1x, small globe | −0.05–0.05 | 0.03758 pass | −0.02069 pass | 0.009631 pass |
| land relief intermittency C1, three globes | 0.08–0.18 | 0.07497 gap | 0.06095 gap | 0.05768 **fail** (marker off) |

Read with intervals:

| reading | gates? | main | #73 | #75 |
|---|---|---|---|---|
| hypsometric integral, small globe | gates | 0.361 [0.332, 0.388] in | 0.351 [0.308, 0.385] straddles | 0.336 [0.316, 0.359] straddles |
| drainage area exceedance exponent, small globe | advisory | 0.479 [0.422, 0.561] | 0.422 [0.343, 0.467] | 0.389 [0.358, 0.470] |
| discharge exceedance exponent, small globe | advisory | 0.438 [0.383, 0.553] | 0.391 [0.256, 0.588] | 0.403 [0.338, 0.556] |
| Hack exponent, small globe | gates | 0.578 [0.558, 0.598] in | 0.575 [0.552, 0.598] in | 0.550 [0.516, 0.583] straddles |
| Hack exponent, globe | gates | 0.587 [0.578, 0.596] in | 0.590 [0.589, 0.594] in | 0.601 [0.592, 0.601] straddles |
| ridge-valley wavelength, small globe | gates | 133 [133, 320] straddles | 229 [145, 400] straddles | 160 [133, 200] in |
| channel concavity, small globe | advisory | 0.233 [0.071, 0.336] out | 0.257 [0.212, 0.309] out | 0.330 [0.266, 0.538] |
| Flint's law fit R2, small globe | advisory | 0.925 [0.790, 1.061] | 0.845 [0.529, 1.16] | 0.950 [0.822, 1.08] |
| Hack exponent, 2x less 1x, small globe | gates | 0.014 [−0.027, 0.056] straddles | −0.001 [−0.045, 0.043] in | 0.047 [0.007, 0.087] straddles |
| channel concavity, 2x less 1x, small globe | advisory | 0.172 [−0.068, 0.431] | 0.119 [0.021, 0.162] | −0.058 [−0.187, 0.109] |
| hypsometric integral, 2x less 1x, small globe | gates | 0.019 [−0.008, 0.071] straddles | 0.002 [−0.049, 0.044] in | 0.004 [−0.041, 0.020] in |
| land relief intermittency C1, three globes | gates | 0.067 [0.063, 0.099] gap | 0.056 [0.055, 0.072] gap | 0.063 [0.045, 0.069] **out** |

Against main, by the comparison rule above, **every reading on both branches
is the same within the noise**: no interval is clear of main's. Read pooled
on their old seeds, #73 swapped four readings and #75 six (seven with C1,
whose marker it took off). Read with
intervals, no gating river or relief reading fails on either branch. The one
exception is #75's C1: #75 took the gap marker off at a pooled 0.087 (on its
base). Here, all three of its globes read under the band, so by the seeds the
gap has not closed. Main is in the same place, only with the marker still on.

The failures of the whole yardstick run, with these yardsticks:

| | main | #73 | #75 |
|---|---|---|---|
| midlatitude over subtropical rain, globe | fail | fail | pass |
| mean land rain, 2x over 1x, small globe | fail | fail | fail |
| land share of Aridisols | fail | fail | fail |
| land share of Gelisols | fail | fail | fail |
| land relief intermittency C1, three globes | gap | gap | fail (marker off) |

Before this change, main failed eight: those four, plus the hypsometric
integral, the concavity and the two exceedance exponents of the small
globes.

## Cost

No new small globe or full globe. The small-globe readings read the sixteen
globes the exceedance exponents already made. The globe's Hack exponent
reads the three full globes the shape yardsticks already make. The
resolution readings read eight pairs where they read four, which is four
more double globes (512x256): about 20 s each where their histories are not
kept, and seconds where they are.

`go test -run 'TestRealNumbers|TestTheRealWorld' -count=1 -timeout 60m .` on
main 2c51bea, with every history kept, back to back on 2026-10-07: 79.9 s
before, 100.4 s after. A run that has to make every world (a change outside
the tests) adds the four double globes, some 80 s more. See
`docs/perf/suite.md`.

## Reproducing the tables

```bash
TERRA_YARDSTICK_SPREAD=spread.json go test -run TestTheReadingsSpread -v -timeout 60m .
```

This logs and writes every seeded reading both ways: the median with its
interval and the pooled fit with its jackknife, the per-seed sd, the seeds
for a tenth, and the reading as it was before. To judge a branch, run it
there with these test files and compare the `reading` intervals by the rule
above.
