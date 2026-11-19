package kms

import (
	"image"
	"os"
	"syscall"
	"testing"

	"github.com/BeatGlow/display"
	"github.com/BeatGlow/display/internal/buffer"
	"github.com/BeatGlow/display/internal/ioctl"
)

// BenchmarkMappedRotation90 alternates between two private dumb buffers on
// the selected DRM card. Neither buffer is registered for scanout.
func BenchmarkMappedRotation90(b *testing.B) {
	card := os.Getenv("DISPLAY_BENCH_DRM_CARD")
	if card == "" {
		b.Skip("set DISPLAY_BENCH_DRM_CARD to benchmark a DRM device")
	}
	file, err := os.OpenFile(card, os.O_RDWR, 0)
	if err != nil {
		b.Fatal(err)
	}
	// Closing the file releases its dumb-buffer handles.
	b.Cleanup(func() {
		if err := file.Close(); err != nil {
			b.Error(err)
		}
	})

	const width, height = 1280, 400
	bounds := image.Rect(0, 0, width, height)
	src := image.NewRGBA(image.Rect(0, 0, height, width))
	for i := range src.Pix {
		src.Pix[i] = byte(i)
	}

	var buffers [2]scanoutBuffer
	for i := range buffers {
		create := dumbBuffer{width: width, height: height, bpp: 32}
		if err := ioctl.Do(file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &create, drmModeCreateDumb), &create); err != nil {
			b.Fatal(err)
		}
		mapping := dumbMap{handle: create.handle}
		if err := ioctl.Do(file.Fd(), ioctl.Pointer(ioctl.Read|ioctl.Write, &mapping, drmModeMapDumb), &mapping); err != nil {
			b.Fatal(err)
		}
		pix, err := syscall.Mmap(int(file.Fd()), int64(mapping.offset), int(create.size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(func() {
			if err := syscall.Munmap(pix); err != nil {
				b.Error(err)
			}
		})
		buffers[i] = scanoutBuffer{pix: pix, pitch: create.pitch}
	}

	cpuMicros := func() int64 {
		var usage syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
			b.Fatal(err)
		}
		return usage.Utime.Nano()/1000 + usage.Stime.Nano()/1000
	}
	scratch := make([]byte, width*height*4)
	row := make([]byte, width*4)
	b.ResetTimer()
	start := cpuMicros()
	for i := 0; i < b.N; i++ {
		dst := &buffers[i%2]
		if err := buffer.CopyToXRGBRotatedInto(dst.pix, int(dst.pitch), src, bounds, display.Rotate90, scratch, row); err != nil {
			b.Fatal(err)
		}
	}
	used := cpuMicros() - start
	b.StopTimer()
	b.ReportMetric(float64(used)/float64(b.N), "cpu-us/frame")
}
