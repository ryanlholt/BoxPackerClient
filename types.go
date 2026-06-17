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

	// DisableQuantityShortCircuit turns off the large-quantity replication
	// optimisation, which is on by default. The optimisation keeps packing fast
	// for big orders of mixed item types, not just bulk runs of a single item.
	DisableQuantityShortCircuit bool `json:"disableQuantityShortCircuit"`

	// Objective selects which box wins at each packing iteration (boxpacker
	// v0.4.0's custom PackedBoxSorter). Accepted values (case-insensitive):
	//
	//	""/"default"        most items, then fullest by volume (library default)
	//	"billableWeight"    minimise each parcel's billable shipping weight,
	//	                    i.e. max(actual gross weight, dimensional weight)
	//
	// "billableWeight" requires DimWeightDivisor. Note the solver stays greedy:
	// this changes the per-parcel choice, not the global cost across parcels.
	Objective string `json:"objective"`

	// DimWeightDivisor is the carrier's dimensional divisor: dim weight is
	// outerVolume / divisor. It is required when Objective is "billableWeight"
	// and, whenever positive, also populates the volumetricWeight/billableWeight
	// fields on each output box. Dimensions, divisor and item weights must share
	// consistent units (e.g. mm with 5000 → grams; inches with 139 → pounds).
	DimWeightDivisor float64 `json:"dimWeightDivisor"`
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
	Reference         string  `json:"reference"`
	ItemCount         int     `json:"itemCount"`
	Weight            int     `json:"weight"`     // including the empty box
	ItemWeight        int     `json:"itemWeight"` // items only
	InnerVolume       int     `json:"innerVolume"`
	UsedVolume        int     `json:"usedVolume"`
	VolumeUtilisation float64 `json:"volumeUtilisation"`
	// VolumetricWeight and BillableWeight are reported only when the request
	// supplies a positive DimWeightDivisor. BillableWeight is what a carrier
	// would charge: max(actual gross weight, volumetric weight).
	VolumetricWeight float64      `json:"volumetricWeight,omitempty"`
	BillableWeight   float64      `json:"billableWeight,omitempty"`
	Items            []ItemOutput `json:"items"`
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

// applyObjective configures the packer's box-selection strategy from the
// request options, validating the objective and its parameters.
func applyObjective(p *boxpacker.Packer, o Options) error {
	switch strings.ToLower(strings.TrimSpace(o.Objective)) {
	case "", "default":
		// Leave the library default (most items, then fullest) in place.
		return nil
	case "billableweight", "billable", "dimweight":
		if o.DimWeightDivisor <= 0 {
			return fmt.Errorf("objective %q requires a positive dimWeightDivisor", o.Objective)
		}
		p.SetPackedBoxSorter(billableWeightSorter(o.DimWeightDivisor))
		return nil
	default:
		return fmt.Errorf("unknown objective %q (want default or billableWeight)", o.Objective)
	}
}

// billableWeightSorter prefers the box with the lower billable shipping weight
// (max of actual and dimensional weight). Ties fall back to the default
// objective's intent: fewer parcels (more items per box), then fuller by volume.
func billableWeightSorter(divisor float64) boxpacker.PackedBoxSorter {
	return boxpacker.PackedBoxSorterFunc(func(a, b *boxpacker.PackedBox) int {
		aw, bw := boxpacker.BillableWeight(a, divisor), boxpacker.BillableWeight(b, divisor)
		switch {
		case aw < bw:
			return -1
		case aw > bw:
			return 1
		}
		if d := len(b.Items) - len(a.Items); d != 0 {
			return d // more items first
		}
		switch {
		case a.VolumeUtilisation() > b.VolumeUtilisation():
			return -1
		case a.VolumeUtilisation() < b.VolumeUtilisation():
			return 1
		default:
			return 0
		}
	})
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

	if err := applyObjective(p, r.Options); err != nil {
		return err
	}

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

// newPackedBoxOutput converts a library PackedBox to its JSON output form. When
// divisor is positive, the volumetric and billable weight fields are populated
// (using the carrier's dimensional divisor); otherwise they are left zero and
// omitted from the JSON.
func newPackedBoxOutput(b *boxpacker.PackedBox, divisor float64) PackedBoxOutput {
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
	out := PackedBoxOutput{
		Reference:         b.Box.Reference(),
		ItemCount:         len(b.Items),
		Weight:            b.Weight(),
		ItemWeight:        b.ItemWeight(),
		InnerVolume:       b.InnerVolume(),
		UsedVolume:        b.UsedVolume(),
		VolumeUtilisation: b.VolumeUtilisation(),
		Items:             items,
	}
	if divisor > 0 {
		out.VolumetricWeight = boxpacker.VolumetricWeight(b.Box, divisor)
		out.BillableWeight = boxpacker.BillableWeight(b, divisor)
	}
	return out
}
