package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mailerBox() BoxInput {
	return BoxInput{
		Reference: "mailer", OuterWidth: 230, OuterLength: 300, OuterDepth: 240,
		EmptyWeight: 160, InnerWidth: 220, InnerLength: 290, InnerDepth: 230, MaxWeight: 15000,
	}
}

func TestPackFitsEverythingInOneBox(t *testing.T) {
	req := &Request{
		Boxes: []BoxInput{mailerBox()},
		Items: []ItemInput{
			{Description: "toy", Width: 80, Length: 60, Depth: 60, Weight: 150, Rotation: "best", Quantity: 4},
		},
	}

	resp, err := Pack(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected packing error: %s", resp.Error)
	}
	if len(resp.Boxes) != 1 {
		t.Fatalf("expected 1 box, got %d", len(resp.Boxes))
	}
	if got := resp.Boxes[0].ItemCount; got != 4 {
		t.Fatalf("expected 4 items packed, got %d", got)
	}
	if len(resp.UnpackedItems) != 0 {
		t.Fatalf("expected nothing unpacked, got %d", len(resp.UnpackedItems))
	}
}

// TestPackLargeMixedQuantity exercises boxpacker v0.3.0's headline capability:
// large quantities of several DISTINCT item types pack correctly and fast,
// thanks to the quantity short-circuit replicating whole boxfuls of the winning
// mix. Before v0.3.0 the short-circuit only fired for a single repeated item.
func TestPackLargeMixedQuantity(t *testing.T) {
	const perType = 4000
	req := &Request{
		Boxes: []BoxInput{{
			Reference: "cube", OuterWidth: 110, OuterLength: 110, OuterDepth: 110,
			EmptyWeight: 100, InnerWidth: 100, InnerLength: 100, InnerDepth: 100, MaxWeight: 100000,
		}},
		Items: []ItemInput{
			{Description: "A", Width: 50, Length: 50, Depth: 50, Weight: 50, Rotation: "best", Quantity: perType},
			{Description: "B", Width: 40, Length: 40, Depth: 40, Weight: 40, Rotation: "best", Quantity: perType},
			{Description: "C", Width: 30, Length: 30, Depth: 30, Weight: 30, Rotation: "best", Quantity: perType},
		},
	}

	start := time.Now()
	resp, err := Pack(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected packing error: %s", resp.Error)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("mixed pack took %s, expected the short-circuit to keep it fast", elapsed)
	}

	total := 0
	for _, b := range resp.Boxes {
		total += b.ItemCount
	}
	if want := 3 * perType; total != want {
		t.Fatalf("expected %d items packed across %d boxes, got %d", want, len(resp.Boxes), total)
	}
	if len(resp.UnpackedItems) != 0 {
		t.Fatalf("expected nothing unpacked, got %d", len(resp.UnpackedItems))
	}
}

func TestPackPartialResultsKeepsLeftovers(t *testing.T) {
	req := &Request{
		Boxes: []BoxInput{mailerBox()},
		Items: []ItemInput{
			{Description: "fits", Width: 80, Length: 60, Depth: 60, Weight: 150, Quantity: 1},
			{Description: "huge", Width: 9000, Length: 9000, Depth: 9000, Weight: 150, Quantity: 1},
		},
		Options: Options{AllowPartialResults: true},
	}

	resp, err := Pack(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("partial results should not report an error, got %q", resp.Error)
	}
	if len(resp.UnpackedItems) != 1 || resp.UnpackedItems[0].Description != "huge" {
		t.Fatalf("expected 'huge' left unpacked, got %+v", resp.UnpackedItems)
	}
}

func TestPackUnpackableItemReportsError(t *testing.T) {
	req := &Request{
		Boxes: []BoxInput{mailerBox()},
		Items: []ItemInput{
			{Description: "huge", Width: 9000, Length: 9000, Depth: 9000, Weight: 150, Quantity: 1},
		},
	}

	resp, err := Pack(req)
	if err != nil {
		t.Fatalf("validation error not expected: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected a packing error for an item that fits in no box")
	}
}

func TestPackValidationErrors(t *testing.T) {
	cases := map[string]*Request{
		"no boxes": {Items: []ItemInput{{Description: "x", Width: 1, Length: 1, Depth: 1}}},
		"no items": {Boxes: []BoxInput{mailerBox()}},
		"bad rotation": {
			Boxes: []BoxInput{mailerBox()},
			Items: []ItemInput{{Description: "x", Width: 1, Length: 1, Depth: 1, Rotation: "sideways"}},
		},
		"missing box reference": {
			Boxes: []BoxInput{{OuterWidth: 1, InnerWidth: 1, InnerLength: 1, InnerDepth: 1, MaxWeight: 1}},
			Items: []ItemInput{{Description: "x", Width: 1, Length: 1, Depth: 1}},
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Pack(req); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestParseRotation(t *testing.T) {
	for _, in := range []string{"", "best", "BestFit", "6", "keepFlat", "FLAT", "2", "never", "1"} {
		if _, err := parseRotation(in); err != nil {
			t.Errorf("parseRotation(%q) errored: %v", in, err)
		}
	}
	if _, err := parseRotation("nope"); err == nil {
		t.Error("expected error for unknown rotation")
	}
}

func TestRunStdioRoundTrip(t *testing.T) {
	input := `{"boxes":[{"reference":"mailer","innerWidth":220,"innerLength":290,"innerDepth":230,"maxWeight":15000}],
	           "items":[{"description":"toy","width":80,"length":60,"depth":60,"weight":150,"quantity":2}]}`
	var out bytes.Buffer
	if err := runStdio(strings.NewReader(input), &out, false); err != nil {
		t.Fatalf("runStdio: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(resp.Boxes) != 1 || resp.Boxes[0].ItemCount != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
