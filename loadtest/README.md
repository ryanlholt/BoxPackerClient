# Load testing boxpackerclient

`POST /pack` is **CPU-bound** (the bin-packing computation), stateless, and does no
I/O. So the bottleneck is CPU cores, and per-request cost scales with the number of
items/boxes in the payload. A meaningful test therefore (a) varies payload size and
(b) drives a controlled request **rate**, not just raw connection count.

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
```

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

(Add a `BenchmarkPack` to a `_test.go` file that calls `Pack(&req)` directly.)

## Interpreting results / what to watch

- **Find the knee:** sweep `RATE` (200 → 400 → 600 …) until p99 latency blows
  past your SLO. The last good rate is per-instance capacity → divide your target
  peak load by it to size horizontal replicas.
- **CPU is the wall:** `GOMAXPROCS`/`--cpus` caps throughput linearly. There's no
  connection pool or DB to tune — scale out, or speed up `boxpacker.Pack()`.
- **Load-generate from another machine** for high rates; a local k6 competes with
  the server for the same cores and skews latency upward.
