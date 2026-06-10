package main

import (
	boxpacker "github.com/ryanlholt/BoxPacker"
)

// Pack runs a single packing problem and returns its result.
//
// A validation failure (bad input) is returned as an error so callers can
// distinguish it from a packing outcome. A packing failure (an item that fits
// in no box, when partial results are not allowed) is reported in
// Response.Error with no error returned, so the partially-useful response can
// still be delivered.
func Pack(req *Request) (*Response, error) {
	packer := boxpacker.NewPacker()
	if err := req.applyTo(packer); err != nil {
		return nil, err
	}

	resp := &Response{}

	packedBoxes, err := packer.Pack()
	for _, b := range packedBoxes {
		resp.Boxes = append(resp.Boxes, newPackedBoxOutput(b))
	}

	if err != nil {
		resp.Error = err.Error()
	}

	// Surface anything left unpacked (only non-empty when partial results are
	// allowed, or alongside an error).
	for _, item := range packer.UnpackedItems() {
		resp.UnpackedItems = append(resp.UnpackedItems, ItemOutput{
			Description: item.Description(),
			Width:       item.Width(),
			Length:      item.Length(),
			Depth:       item.Depth(),
		})
	}

	return resp, nil
}
