//go:build amd64

package buffer

import (
	"runtime"
	"sync"
)

func rotateXRGB90(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 != 0 || height&3 != 0 {
		rotateXRGB90Go(dst, dstStride, src, srcStride, width, height)
		return
	}
	parallelRotateXRGBAMD64(dst, dstStride, src, srcStride, width, height, rotateXRGB90AMD64)
}

func rotateXRGB270(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 != 0 || height&3 != 0 {
		rotateXRGB270Go(dst, dstStride, src, srcStride, width, height)
		return
	}
	parallelRotateXRGBAMD64(dst, dstStride, src, srcStride, width, height, rotateXRGB270AMD64)
}

func rotateXRGB180(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 != 0 {
		rotateXRGB180Go(dst, dstStride, src, srcStride, width, height)
		return
	}
	parallelRotateXRGBAMD64(dst, dstStride, src, srcStride, width, height, rotateXRGB180AMD64)
}

func rotateRGBA180(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 {
		return false
	}
	parallelRotateXRGBAMD64(dst, dstStride, src, srcStride, width, height, rotateRGBA180AMD64)
	return true
}

func rotateRGBA90(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 || height&3 != 0 {
		return false
	}
	parallelRotateXRGBAMD64(dst, dstStride, src, srcStride, width, height, rotateRGBA90AMD64)
	return true
}

func rotateRGBA270(dst []byte, dstStride int, src []byte, srcStride, width, height int) bool {
	if width&3 != 0 || height&3 != 0 {
		return false
	}
	parallelRotateXRGBAMD64(dst, dstStride, src, srcStride, width, height, rotateRGBA270AMD64)
	return true
}

type rotateAMD64Func func(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)

func parallelRotateXRGBAMD64(dst []byte, dstStride int, src []byte, srcStride, width, height int, rotate rotateAMD64Func) {
	workers := min(runtime.GOMAXPROCS(0), height/64)
	if workers < 2 {
		rotate(dst, dstStride, src, srcStride, width, height, 0, height)
		return
	}
	rowsPerWorker := ((height/4+workers-1)/workers + 3) / 4 * 4
	var wg sync.WaitGroup
	for startY := 0; startY < height; startY += rowsPerWorker {
		endY := min(startY+rowsPerWorker, height)
		wg.Add(1)
		go func(startY, endY int) {
			defer wg.Done()
			rotate(dst, dstStride, src, srcStride, width, height, startY, endY)
		}(startY, endY)
	}
	wg.Wait()
}

func rotateXRGB90AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateXRGB270AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateXRGB180AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA90AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA270AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateRGBA180AMD64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
