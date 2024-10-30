package stbresize

/*
#cgo CFLAGS: -msse -msse2 -msse3 -mssse3 -msse4 -msse4.1 -msse4.2 -mavx -mavx2 -O3
#define STB_IMAGE_RESIZE_IMPLEMENTATION
#include "stb_image_resize2.h"

inline void resize_uint8(void *input_pixels , int input_w , int input_h, int input_stride_in_bytes,
                	void *output_pixels, int output_w, int output_h, int output_stride_in_bytes,
                	stbir_pixel_layout pixel_type ){
	stbir_resize_uint8_linear( (const unsigned char *) input_pixels, input_w, input_h, input_stride_in_bytes,
	 						(unsigned char *) output_pixels, output_w, output_h, output_stride_in_bytes, pixel_type );
}
*/
import "C"

import (
	"image"
	"unsafe"
)

func StbirResizeUint8LinearRGBA(img *image.RGBA, dest *image.RGBA, r image.Rectangle) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	C.resize_uint8((unsafe.Pointer)(&img.Pix[0]), C.int(width), C.int(height), C.int(img.Stride), (unsafe.Pointer)(&dest.Pix[0]),
		C.int(r.Dx()), C.int(r.Dy()), C.int(dest.Stride), C.stbir_pixel_layout(11))
}

func StbirResizeUint8LinearNRGBA(img *image.NRGBA, dest *image.NRGBA, r image.Rectangle) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	C.resize_uint8((unsafe.Pointer)(&img.Pix[0]), C.int(width), C.int(height), C.int(img.Stride), (unsafe.Pointer)(&dest.Pix[0]),
		C.int(r.Dx()), C.int(r.Dy()), C.int(dest.Stride), C.stbir_pixel_layout(4))
}
