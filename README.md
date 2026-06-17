# boxpackerclient

A small Go client around the
[boxpacker](https://github.com/ryanlholt/BoxPacker) library. It reads a
packing problem as JSON and returns the solution as JSON, either over
**stdin/stdout** or as an **HTTP service**.

## Build

```sh
go build -o boxpackerclient .
```

## Docker (HTTP service)

```sh
docker build -t boxpackerclient .
docker run --rm -p 8080:8080 boxpackerclient
curl -s --data-binary @example.json localhost:8080/pack
```

## Usage

### stdin / stdout (default)

```sh
boxpackerclient < example.json            # compact JSON
boxpackerclient -pretty < example.json    # indented JSON
```

`example-bulk.json` is a larger sample: thousands of items across several
distinct types, packed in one request. It exercises the large-quantity
short-circuit (see `disableQuantityShortCircuit` below) and still returns
near-instantly.

### HTTP service

```sh
boxpackerclient -http :8080
curl -s --data-binary @example.json localhost:8080/pack
```

Endpoints:

| Method | Path       | Description                         |
|--------|------------|-------------------------------------|
| POST   | `/pack`    | Pack the JSON body, return solution |
| GET    | `/healthz` | Liveness check (`ok`)               |

## Request schema

```json
{
  "boxes": [
    {
      "reference": "small mailer",
      "outerWidth": 230, "outerLength": 300, "outerDepth": 240,
      "emptyWeight": 160,
      "innerWidth": 220, "innerLength": 290, "innerDepth": 230,
      "maxWeight": 15000,
      "quantityAvailable": 0
    }
  ],
  "items": [
    {
      "description": "mug",
      "width": 110, "length": 110, "depth": 105,
      "weight": 350,
      "rotation": "never",
      "quantity": 4
    }
  ],
  "options": {
    "allowPartialResults": false,
    "disableQuantityShortCircuit": false,
    "objective": "default",
    "dimWeightDivisor": 0
  }
}
```

- **Dimensions and weights** are integers and unit-agnostic — just be
  consistent (millimetres and grams are recommended).
- **`rotation`** is `"best"` (any orientation, the default), `"keepFlat"`
  (may turn 90° but not on its side), or `"never"` (exact orientation only).
  The numeric library values `6`/`2`/`1` are also accepted.
- **`quantityAvailable`** limits how many of a box type may be used; `0` or
  omitted means unlimited.
- **`quantity`** defaults to `1`.
- **`allowPartialResults`** — when `true`, items that fit in no box are
  returned in `unpackedItems` instead of failing the whole request.
- **`disableQuantityShortCircuit`** turns off the large-quantity replication
  optimisation (on by default). The optimisation keeps packing fast for big
  orders of *mixed* item types — not just bulk runs of one item — so there is
  rarely a reason to disable it outside of debugging.
- **`objective`** selects which box wins at each packing step (boxpacker
  v0.4.0's custom box sorter). `"default"` (or omitted) keeps the built-in
  order — most items, then fullest. `"billableWeight"` instead minimises each
  parcel's *billable shipping weight*: the greater of its actual gross weight
  and its dimensional weight, so it avoids large, lightly-filled boxes that a
  carrier would over-charge for. The solver stays greedy, so this tunes the
  per-parcel choice, not the global cost across all parcels.
- **`dimWeightDivisor`** is the carrier's dimensional divisor (dim weight =
  `outerVolume / divisor`). It is **required** when `objective` is
  `"billableWeight"`. Whenever it is positive, each output box also reports its
  `volumetricWeight` and `billableWeight`. Keep the divisor consistent with your
  dimension/weight units (e.g. millimetres with `5000`, inches with `139`).

## Response schema

```json
{
  "boxes": [
    {
      "reference": "small mailer",
      "itemCount": 16,
      "weight": 3960,
      "itemWeight": 3800,
      "innerVolume": 14674000,
      "usedVolume": 9600000,
      "volumeUtilisation": 65.42,
      "volumetricWeight": 2934.8,
      "billableWeight": 3960,
      "items": [
        { "description": "mug", "x": 0, "y": 0, "z": 0, "width": 110, "length": 110, "depth": 105 }
      ]
    }
  ],
  "unpackedItems": [],
  "error": ""
}
```

- Each item's `x`/`y`/`z` is the corner closest to the box origin, and
  `width`/`length`/`depth` are the item's dimensions **in its packed
  orientation** (which may differ from the input if it was rotated).
- `volumetricWeight` and `billableWeight` appear only when the request supplied a
  positive `dimWeightDivisor`; otherwise they are omitted.
- `error` is set (with no HTTP error / non-zero exit) when an item fits in no
  box and `allowPartialResults` is `false`; any boxes packed before the
  failure are still returned. Malformed input is a hard failure instead:
  exit code 1 on the CLI, HTTP 400 from the service.

## Test

```sh
go test ./...
```
