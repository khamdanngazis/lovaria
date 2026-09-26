package imageproc

import (
	"image"
	"image/color"
	"image/draw"
)

// pixelRow mengisi dst (RGBA premultiplied 8-bit, 4 byte/piksel) dengan baris y
// gambar sumber. Dispesialisasi per tipe supaya tidak lewat At() per piksel.
type pixelRow func(y int, dst []byte)

func rowReader(src image.Image) (pixelRow, int, int) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	switch s := src.(type) {
	case *image.YCbCr: // hasil decode JPEG
		return func(y int, dst []byte) {
			sy := b.Min.Y + y
			for x := 0; x < w; x++ {
				sx := b.Min.X + x
				yi, ci := s.YOffset(sx, sy), s.COffset(sx, sy)
				r, g, bl := color.YCbCrToRGB(s.Y[yi], s.Cb[ci], s.Cr[ci])
				dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = r, g, bl, 0xFF
			}
		}, w, h
	case *image.RGBA:
		return func(y int, dst []byte) {
			i := s.PixOffset(b.Min.X, b.Min.Y+y)
			copy(dst, s.Pix[i:i+4*w])
		}, w, h
	case *image.NRGBA: // PNG/WebP dengan alpha → premultiply
		return func(y int, dst []byte) {
			i := s.PixOffset(b.Min.X, b.Min.Y+y)
			for x := 0; x < w; x++ {
				p := s.Pix[i+4*x : i+4*x+4]
				a := uint32(p[3])
				dst[4*x] = uint8(uint32(p[0]) * a / 0xFF)   //nolint:gosec // G115: ≤ 255
				dst[4*x+1] = uint8(uint32(p[1]) * a / 0xFF) //nolint:gosec // G115: ≤ 255
				dst[4*x+2] = uint8(uint32(p[2]) * a / 0xFF) //nolint:gosec // G115: ≤ 255
				dst[4*x+3] = p[3]
			}
		}, w, h
	default: // tipe lain (paletted, gray, ...): konversi sekali ke RGBA
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
		return rowReader(rgba)
	}
}

// span adalah kontribusi piksel sumber [start, start+len(weights)) ke satu piksel tujuan.
type span struct {
	start   int
	weights []float32
}

// spans menghitung jejak area setiap piksel tujuan pada sumbu sumber (box filter).
func spans(srcLen, dstLen int) []span {
	out := make([]span, dstLen)
	scale := float64(srcLen) / float64(dstLen)
	for d := 0; d < dstLen; d++ {
		lo, hi := float64(d)*scale, float64(d+1)*scale
		start := int(lo)
		end := min(int(hi+0.999999), srcLen)
		ws := make([]float32, 0, end-start)
		for s := start; s < end; s++ {
			w := min(hi, float64(s+1)) - max(lo, float64(s))
			ws = append(ws, float32(w/scale))
		}
		out[d] = span{start: start, weights: ws}
	}
	return out
}

// resizeArea memperkecil src ke dw×dh dengan rata-rata area (tanpa aliasing),
// meratakan transparansi ke latar putih. Memori tambahan hanya O(lebar).
func resizeArea(src image.Image, dw, dh int) *image.RGBA {
	read, sw, sh := rowReader(src)
	xs, ys := spans(sw, dw), spans(sh, dh)
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	row := make([]byte, 4*sw)
	acc := make([]float32, 4*dw)

	for dy := 0; dy < dh; dy++ {
		clear(acc)
		sp := ys[dy]
		for k, wy := range sp.weights {
			read(sp.start+k, row)
			for dx, sx := range xs {
				var r, g, b, a float32
				for j, wx := range sx.weights {
					p := row[4*(sx.start+j):]
					r += wx * float32(p[0])
					g += wx * float32(p[1])
					b += wx * float32(p[2])
					a += wx * float32(p[3])
				}
				o := 4 * dx
				acc[o] += wy * r
				acc[o+1] += wy * g
				acc[o+2] += wy * b
				acc[o+3] += wy * a
			}
		}
		// Latar putih: warna premultiplied + (1 - alpha) × 255.
		out := dst.Pix[dy*dst.Stride : dy*dst.Stride+4*dw]
		for dx := 0; dx < dw; dx++ {
			o := 4 * dx
			bg := 255 - acc[o+3]
			out[o] = clamp8(acc[o] + bg)
			out[o+1] = clamp8(acc[o+1] + bg)
			out[o+2] = clamp8(acc[o+2] + bg)
			out[o+3] = 0xFF
		}
	}
	return dst
}

func clamp8(v float32) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	}
	return uint8(v + 0.5)
}
