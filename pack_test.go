package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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
