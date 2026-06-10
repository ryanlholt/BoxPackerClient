package main

import (
	"fmt"
	"strings"

	boxpacker "github.com/ryanlholt/BoxPacker"
)

// Request is the JSON payload describing a packing problem.
type Request struct {
	Boxes   []BoxInput  `json:"boxes"`
	Items   []ItemInput `json:"items"`
	Options Options     `json:"options"`
}

// Options tweak the packer's behaviour. The zero value matches the library
// defaults except for QuantityShortCircuit, which defaults to enabled (see
// applyTo).
type Options struct {
	// AllowPartialResults, when true, returns the boxes that could be packed
	// instead of erroring on an unpackable item; leftovers appear in
	// Response.UnpackedItems.
	AllowPartialResults bool `json:"allowPartialResults"`

	// DisableQuantityShortCircuit turns off the identical-item replication
	// optimisation, which is on by default.
	DisableQuantityShortCircuit bool `json:"disableQuantityShortCircuit"`
}

// BoxInput describes one available box type.
type BoxInput struct {
	Reference   string `json:"reference"`
	OuterWidth  int    `json:"outerWidth"`
	OuterLength int    `json:"outerLength"`
	OuterDepth  int    `json:"outerDepth"`
	EmptyWeight int    `json:"emptyWeight"`
	InnerWidth  int    `json:"innerWidth"`
	InnerLength int    `json:"innerLength"`
	InnerDepth  int    `json:"innerDepth"`
	MaxWeight   int    `json:"maxWeight"`
	// QuantityAvailable limits how many of this box type may be used. Zero or
	// negative means unlimited.
	QuantityAvailable int `json:"quantityAvailable"`
}

// ItemInput describes a group of identical items to pack.
type ItemInput struct {
	Description string `json:"description"`
	Width       int    `json:"width"`
	Length      int    `json:"length"`
	Depth       int    `json:"depth"`
	Weight      int    `json:"weight"`
	// Rotation accepts "best", "keepFlat" or "never" (case-insensitive), or
	// the numeric library values 6/2/1. Empty defaults to "best".
	Rotation string `json:"rotation"`
	// Quantity of this item to pack. Defaults to 1 when zero or negative.
	Quantity int `json:"quantity"`
}

// Response is the JSON result of a packing run.
type Response struct {
	Boxes         []PackedBoxOutput `json:"boxes"`
	UnpackedItems []ItemOutput      `json:"unpackedItems,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// PackedBoxOutput is one box in the solution along with its contents and a few
// derived statistics.
type PackedBoxOutput struct {
	Reference         string       `json:"reference"`
	ItemCount         int          `json:"itemCount"`
	Weight            int          `json:"weight"`     // including the empty box
	ItemWeight        int          `json:"itemWeight"` // items only
	InnerVolume       int          `json:"innerVolume"`
	UsedVolume        int          `json:"usedVolume"`
	VolumeUtilisation float64      `json:"volumeUtilisation"`
	Items             []ItemOutput `json:"items"`
}

// ItemOutput is one packed item: its identity plus the position and
// orientation it was packed in. Position/orientation fields are omitted for
// unpacked items.
type ItemOutput struct {
	Description string `json:"description"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Z           int    `json:"z"`
	Width       int    `json:"width"`
	Length      int    `json:"length"`
	Depth       int    `json:"depth"`
}

// parseRotation maps the JSON rotation field to a boxpacker.Rotation.
func parseRotation(s string) (boxpacker.Rotation, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "best", "bestfit", "6":
		return boxpacker.RotationBestFit, nil
	case "keepflat", "flat", "2":
		return boxpacker.RotationKeepFlat, nil
	case "never", "1":
		return boxpacker.RotationNever, nil
	default:
		return 0, fmt.Errorf("unknown rotation %q (want best, keepFlat or never)", s)
	}
}

// applyTo populates a Packer from the request, validating as it goes.
func (r *Request) applyTo(p *boxpacker.Packer) error {
	if len(r.Boxes) == 0 {
		return fmt.Errorf("request must contain at least one box")
	}
	if len(r.Items) == 0 {
		return fmt.Errorf("request must contain at least one item")
	}

	p.AllowPartialResults(r.Options.AllowPartialResults)
	p.SetQuantityShortCircuit(!r.Options.DisableQuantityShortCircuit)

	for i, b := range r.Boxes {
		if b.Reference == "" {
			return fmt.Errorf("box %d: reference is required", i)
		}
		if b.QuantityAvailable > 0 {
			p.AddBox(boxpacker.NewLimitedSupplyBox(
				b.Reference,
				b.OuterWidth, b.OuterLength, b.OuterDepth, b.EmptyWeight,
				b.InnerWidth, b.InnerLength, b.InnerDepth, b.MaxWeight,
				b.QuantityAvailable,
			))
		} else {
			p.AddBox(boxpacker.NewBox(
				b.Reference,
				b.OuterWidth, b.OuterLength, b.OuterDepth, b.EmptyWeight,
				b.InnerWidth, b.InnerLength, b.InnerDepth, b.MaxWeight,
			))
		}
	}

	for i, it := range r.Items {
		if it.Description == "" {
			return fmt.Errorf("item %d: description is required", i)
		}
		rotation, err := parseRotation(it.Rotation)
		if err != nil {
			return fmt.Errorf("item %q: %w", it.Description, err)
		}
		qty := it.Quantity
		if qty <= 0 {
			qty = 1
		}
		p.AddItem(boxpacker.NewItem(
			it.Description, it.Width, it.Length, it.Depth, it.Weight, rotation,
		), qty)
	}

	return nil
}

// newPackedBoxOutput converts a library PackedBox to its JSON output form.
func newPackedBoxOutput(b *boxpacker.PackedBox) PackedBoxOutput {
	items := make([]ItemOutput, len(b.Items))
	for i, pi := range b.Items {
		items[i] = ItemOutput{
			Description: pi.Item.Description(),
			X:           pi.X,
			Y:           pi.Y,
			Z:           pi.Z,
			Width:       pi.Width,
			Length:      pi.Length,
			Depth:       pi.Depth,
		}
	}
	return PackedBoxOutput{
		Reference:         b.Box.Reference(),
		ItemCount:         len(b.Items),
		Weight:            b.Weight(),
		ItemWeight:        b.ItemWeight(),
		InnerVolume:       b.InnerVolume(),
		UsedVolume:        b.UsedVolume(),
		VolumeUtilisation: b.VolumeUtilisation(),
		Items:             items,
	}
}
