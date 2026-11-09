//go:build arm64

package buffer

import (
	"runtime"
	"sync"
)

func rotateXRGB90(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 == 0 && height&3 == 0 {
		parallelRotateXRGB(dst, dstStride, src, srcStride, width, height, rotateXRGB90ARM64)
		return
	}
	rotateXRGB90Go(dst, dstStride, src, srcStride, width, height)
}

func rotateXRGB270(dst []byte, dstStride int, src []byte, srcStride, width, height int) {
	if width&3 == 0 && height&3 == 0 {
		parallelRotateXRGB(dst, dstStride, src, srcStride, width, height, rotateXRGB270ARM64)
		return
	}
	rotateXRGB270Go(dst, dstStride, src, srcStride, width, height)
}

type rotateXRGBARM64Func func(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)

func parallelRotateXRGB(dst []byte, dstStride int, src []byte, srcStride, width, height int, rotate rotateXRGBARM64Func) {
	workers := min(runtime.GOMAXPROCS(0), height/4)
	if workers <= 1 {
		rotate(dst, dstStride, src, srcStride, width, height, 0, height)
		return
	}
	var wg sync.WaitGroup
	rowsPerWorker := ((height/4 + workers - 1) / workers) * 4
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

func rotateXRGB90ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
func rotateXRGB270ARM64(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)
