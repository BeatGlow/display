package buffer

import (
	"runtime"
	"sync"
)

type rotateKernel func(dst []byte, dstStride int, src []byte, srcStride, width, height, startY, endY int)

type rotateTask struct {
	dst, src                    []byte
	dstStride, srcStride        int
	width, height, startY, endY int
	kernel                      rotateKernel
}

type rotateWorker struct {
	task  rotateTask
	start chan struct{}
	done  chan struct{}
}

func (w *rotateWorker) run() {
	for range w.start {
		t := &w.task
		t.kernel(t.dst, t.dstStride, t.src, t.srcStride, t.width, t.height, t.startY, t.endY)
		w.task = rotateTask{} // Release frame buffers before the next job arrives.
		w.done <- struct{}{}
	}
}

var rotateWorkers struct {
	mu      sync.Mutex
	workers []*rotateWorker
}

// parallelRotate dispatches disjoint row bands to persistent workers. Keeping
// workers alive avoids allocating a goroutine and closure for every frame.
func parallelRotate(dst []byte, dstStride int, src []byte, srcStride, width, height, minRowsPerWorker int, kernel rotateKernel) {
	workers := min(runtime.GOMAXPROCS(0), height/minRowsPerWorker)
	if workers < 2 {
		kernel(dst, dstStride, src, srcStride, width, height, 0, height)
		return
	}

	rotateWorkers.mu.Lock()
	defer rotateWorkers.mu.Unlock()
	for len(rotateWorkers.workers) < workers {
		w := &rotateWorker{start: make(chan struct{}), done: make(chan struct{})}
		rotateWorkers.workers = append(rotateWorkers.workers, w)
		go w.run()
	}

	rowsPerWorker := ((height/4 + workers - 1) / workers) * 4
	tasks := 0
	for startY := 0; startY < height; startY += rowsPerWorker {
		endY := min(startY+rowsPerWorker, height)
		w := rotateWorkers.workers[tasks]
		w.task = rotateTask{
			dst: dst, src: src,
			dstStride: dstStride, srcStride: srcStride,
			width: width, height: height,
			startY: startY, endY: endY,
			kernel: kernel,
		}
		w.start <- struct{}{}
		tasks++
	}
	for i := 0; i < tasks; i++ {
		<-rotateWorkers.workers[i].done
	}
}
