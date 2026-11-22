/*
Package framebuffer opens a [kms] display when available, then falls back to
Linux [fbdev]. [Open] tries KMS first and reports both backend errors if
neither can be opened.
*/
package framebuffer
