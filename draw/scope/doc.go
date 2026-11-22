/*
Package scope compiles SVG outlines into a repeating oscilloscope signal and
renders it with a phosphor persistence effect. Create a [Renderer] for the
destination bounds, update it from one goroutine, then render it to an image
or a display using [Renderer.BlitToDisplay].
*/
package scope
