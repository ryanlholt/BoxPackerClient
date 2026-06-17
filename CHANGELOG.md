# Changelog

All notable changes to boxpackerclient are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- Upgraded the BoxPacker dependency from `v0.3.0` to `v0.4.0`.

### Added

- **Cost-aware packing (`objective` option).** A new `objective` request option
  selects which box wins at each packing step, backed by BoxPacker v0.4.0's
  custom `PackedBoxSorter`:
  - `"default"` (or omitted) keeps the built-in order — most items, then fullest.
  - `"billableWeight"` minimises each parcel's billable shipping weight, i.e.
    `max(actual gross weight, dimensional weight)`, to avoid large, lightly-filled
    boxes a carrier would over-charge for. Ties fall back to more items per box,
    then fuller by volume. The solver stays greedy, so this tunes the per-parcel
    choice, not the global cost across parcels.
- **`dimWeightDivisor` option.** The carrier's dimensional divisor
  (`dim weight = outerVolume / divisor`). Required when `objective` is
  `"billableWeight"`; rejected (with a validation error) when missing.
- **`volumetricWeight` and `billableWeight` response fields.** Reported on each
  output box whenever a positive `dimWeightDivisor` is supplied; omitted otherwise.
- **Load tests for the new objective:**
  - k6 (`loadtest/pack.js`): new `COST` (default `0.15`) and `DIVISOR` (default
    `5000`) env knobs route a share of generated traffic — both normal and bulk
    payloads — through the cost-aware objective.
  - Go benchmark: `BenchmarkPackObjective` runs the same problem under the
    `default` and `billableWeight` objectives side by side to attribute the custom
    sorter's per-request overhead (measured ~2x the default sorter's cost).
  - Unit tests: `TestPackBillableWeightObjective` (the sorter, not input order,
    picks the lower-billable-weight box and populates the weight fields) and
    `TestPackBillableWeightRequiresDivisor` (validation of a missing divisor and
    unknown objectives).

## [0.3.0] — BoxPacker v0.3.0

### Changed

- Upgraded to BoxPacker v0.3.0, whose quantity short-circuit replicates whole
  boxfuls of the winning mix for large quantities of **mixed** item types (not
  just bulk runs of a single item), keeping such orders fast.

### Added

- Load testing: k6 script (`loadtest/pack.js`) with ramp/rate/spike/soak profiles
  and a `BULK` knob for bulk mixed-item orders, plus `bench_test.go` benchmarks
  for the pure-compute path.
