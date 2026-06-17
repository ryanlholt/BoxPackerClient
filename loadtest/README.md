# Load testing boxpackerclient

`POST /pack` is **CPU-bound** (the bin-packing computation), stateless, and does no
I/O. So the bottleneck is CPU cores. Per-request cost scales with the number of
*distinct box layouts* the packer has to solve — which usually tracks the number of
distinct item shapes and box types, **not** the raw item total: boxpacker v0.3.0
short-circuits large quantities of mixed items by replicating whole boxfuls of the
winning mix. A meaningful test therefore (a) varies payload size and *shape*
(including bulk mixed-item orders, see `BULK` below) and (b) drives a controlled
request **rate**, not just raw connection count.

A slice of traffic also exercises boxpacker v0.4.0's cost-aware
`billableWeight` objective (see `COST`/`DIVISOR` below): a custom box sorter that
runs a comparison per candidate box per iteration to minimise dimensional
shipping weight. It is measurably heavier per request than the default sorter, so
mixing it in keeps the latency numbers honest for a service that packs for cost.

## 1. Start the service

```sh
go build -o boxpackerclient . && ./boxpackerclient -http :8080
# or in Docker:
docker run --rm -p 8080:8080 boxpackerclient
```

Pin the server to a known CPU budget for reproducible numbers, e.g.
`GOMAXPROCS=4 ./boxpackerclient -http :8080` (or `--cpus` in Docker).

## 2. k6 (recommended — realistic mixed payloads & arrival rates)

```sh
brew install k6

k6 run loadtest/pack.js                         # ramp profile (default)
PROFILE=rate RATE=500 k6 run loadtest/pack.js   # hold 500 req/s for 2m
PROFILE=spike k6 run loadtest/pack.js           # sudden 20x spike
PROFILE=soak  RATE=300 k6 run loadtest/pack.js  # 30m steady soak
BULK=0.3 k6 run loadtest/pack.js                # heavier bulk mixed-item mix
COST=0.5 k6 run loadtest/pack.js                # stress the billable-weight sorter
```

By default ~10% of generated requests are **bulk mixed-item orders** (several
distinct item types, each at a large quantity) — the workload v0.3.0's
short-circuit is built for. Tune the share with `BULK` (e.g. `BULK=0` to send
only small/medium orders, `BULK=0.3` to stress the replication path harder).

By default ~15% of requests ask for the **`billableWeight` objective** (boxpacker
v0.4.0's custom box sorter, which optimises for dimensional shipping weight rather
than the default most-items/fullest order). Tune the share with `COST` (`COST=0`
to disable it, `COST=0.5` to make cost-aware packing the dominant cost) and the
carrier divisor with `DIVISOR` (dim weight = `outerVolume / DIVISOR`). This path
is ~2x the per-request CPU of the default sorter, so raising `COST` lowers the
saturation RPS — sweep it to size capacity for a cost-optimising deployment.

Read the output: `http_req_duration` p95/p99 is your latency SLO, `iterations/s`
is sustained throughput, `http_req_failed` should stay ~0. When p99 starts
climbing while RATE rises but throughput flattens, you've found CPU saturation —
that plateau RPS is your single-instance capacity.

## 3. vegeta (quick constant-rate sanity check, single payload)

```sh
brew install vegeta

echo "POST http://localhost:8080/pack" > targets.txt
vegeta attack -targets=targets.txt -body=../example.json \
  -header="Content-Type: application/json" -rate=500 -duration=30s \
  | vegeta report
```

## 4. Go benchmark (pure-compute lower bound, no network)

Measures the packer itself with the network stripped out — useful to know the
theoretical ceiling and to attribute latency to compute vs. HTTP/JSON overhead.

```sh
go test -bench=. -benchmem -benchtime=5s
```

`bench_test.go` calls `Pack(&req)` directly. `BenchmarkPack` sweeps small/medium/
large fixed-quantity problems; `BenchmarkPackLargeMixed` packs large quantities of
several distinct item types to show the v0.3.0 short-circuit keeping cost sublinear
in the item total; `BenchmarkPackObjective` runs the same problem under the default
and v0.4.0 `billableWeight` objectives side by side, so the custom sorter's
per-request overhead is attributable.

## Interpreting results / what to watch

- **Find the knee:** sweep `RATE` (200 → 400 → 600 …) until p99 latency blows
  past your SLO. The last good rate is per-instance capacity → divide your target
  peak load by it to size horizontal replicas.
- **CPU is the wall:** `GOMAXPROCS`/`--cpus` caps throughput linearly. There's no
  connection pool or DB to tune — scale out, or speed up `boxpacker.Pack()`.
- **Load-generate from another machine** for high rates; a local k6 competes with
  the server for the same cores and skews latency upward.
