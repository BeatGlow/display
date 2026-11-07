package display

import (
	"errors"
	"strconv"
	"testing"
)

var spiBenchmarkSink int

type benchmarkSPIWriter struct {
	calls int
	errAt int
}

//go:noinline
func (w *benchmarkSPIWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.errAt != 0 && w.calls == w.errAt {
		return 0, errors.New("write failed")
	}
	return len(data), nil
}

func TestWriteChunkedTo(t *testing.T) {
	writer := &benchmarkSPIWriter{}
	data := make([]byte, 10)
	if err := writeChunkedTo(writer, 4, data); err != nil {
		t.Fatal(err)
	}
	if writer.calls != 3 {
		t.Fatalf("Write calls = %d, want 3", writer.calls)
	}

	writer = &benchmarkSPIWriter{errAt: 2}
	if err := writeChunkedTo(writer, 4, data); err == nil {
		t.Fatal("writeChunkedTo did not return the writer error")
	}
	if writer.calls != 2 {
		t.Fatalf("Write calls after error = %d, want 2", writer.calls)
	}
}

func BenchmarkSPITransferChunking(b *testing.B) {
	data := make([]byte, 400*128*2)
	for _, batchSize := range []uint{4096, 16384} {
		b.Run("Chunked/"+strconv.Itoa(int(batchSize)), func(b *testing.B) {
			writer := &benchmarkSPIWriter{}
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := writeChunkedTo(writer, batchSize, data); err != nil {
					b.Fatal(err)
				}
			}
			spiBenchmarkSink = writer.calls
		})
	}
	b.Run("SingleWrite", func(b *testing.B) {
		writer := &benchmarkSPIWriter{}
		b.SetBytes(int64(len(data)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := writer.Write(data); err != nil {
				b.Fatal(err)
			}
		}
		spiBenchmarkSink = writer.calls
	})
}
