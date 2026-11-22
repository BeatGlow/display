/*
Package framebuffer tries [kms.Open] first, then [fbdev.Open]. [Open] returns
an error containing both backend failures if neither can be opened.
*/
package framebuffer
