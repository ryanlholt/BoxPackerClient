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
# public dependency
docker build -t boxpackerclient .

# private dependency (forwards your SSH agent for the module download)
docker build --ssh default --build-arg PRIVATE_DEP=1 -t boxpackerclient .

docker run --rm -p 8080:8080 boxpackerclient
curl -s --data-binary @example.json localhost:8080/pack
```

## Usage

### stdin / stdout (default)

```sh
boxpackerclient < example.json            # compact JSON
boxpackerclient -pretty < example.json    # indented JSON
```

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
    "disableQuantityShortCircuit": false
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
  optimisation (on by default).

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
- `error` is set (with no HTTP error / non-zero exit) when an item fits in no
  box and `allowPartialResults` is `false`; any boxes packed before the
  failure are still returned. Malformed input is a hard failure instead:
  exit code 1 on the CLI, HTTP 400 from the service.

## Test

```sh
go test ./...
```
