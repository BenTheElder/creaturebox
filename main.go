/*
Copyright 2015 Benjamin Elder ("BenTheElder")

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"image"
	"image/draw"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mobile/app"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/event/touch"
	"golang.org/x/mobile/exp/gl/glutil"
	"golang.org/x/mobile/geom"
	"golang.org/x/mobile/gl"
)

var (
	glctx     gl.Context     // opengl context
	images    *glutil.Images // opengl textures manager
	img       *glutil.Image  // opengl texture
	sz        *size.Event    // for tracking the window size
	sim       *Sim           // the simulation
	onAndroid bool           // true if we are running on android
	onArm     bool           // true if we are running on arm
	onDarwin  bool           // true if we are running on darwin
)

func init() {
	// initialize platform detection booleans
	onAndroid = (runtime.GOOS == "android")
	onArm = (strings.HasPrefix(runtime.GOARCH, "arm"))
	onDarwin = (runtime.GOOS == "darwin")
}

func main() {
	// width and height of the simulation area.
	// this seems to be plenty and smaller areas will be cheaper
	// to run especially on mobile.
	width := 405
	height := 720
	// complementary border thickness
	borderWidth := 16
	sim = NewSim(width, height, borderWidth)
	app.Main(func(a app.App) {
		for e := range a.Events() {
			switch e := a.Filter(e).(type) {
			case lifecycle.Event:
				switch e.Crosses(lifecycle.StageVisible) {
				case lifecycle.CrossOn:
					// we want all OpenGL calls to be on this thread,
					// so lock it.
					runtime.LockOSThread()
					glctx, _ = e.DrawContext.(gl.Context)
					images = glutil.NewImages(glctx)
					// get sim buffer size
					simBounds := sim.CurrentFrame.Bounds()
					// create an image for uploading the sim
					// frames to an opengl texture
					img = images.NewImage(simBounds.Dx(), simBounds.Dy())
					// start rendering
					a.Send(paint.Event{})
				case lifecycle.CrossOff:
					// release resources
					img.Release()
					images.Release()
					glctx = nil
					runtime.UnlockOSThread()
					// if we are on osx/linux etc we want to
					// cleanly exit. On android apps we should
					// not return
					if !onAndroid {
						return
					}
				}
			case size.Event:
				// store for tracking app size and dpi
				sz = &e
			case touch.Event:
				// if the user clicks the screen, spawn a
				// random creature.
				if e.Type == touch.TypeBegin {
					sim.SpawnRandomCreature()
				}
			case paint.Event:
				// can't draw if opengl context doesnt exist.
				if glctx == nil {
					continue
				}
				// update sim
				sim.DoTick()
				// draw to screen
				Draw()
				// tell the mobile package we're done
				a.Publish()
				// keep updating
				a.Send(paint.Event{})
				// TODO: Ugly Hack, rate-limit on desktop
				if !onAndroid && !(onDarwin && onArm) {
					time.Sleep(time.Millisecond * 30)
				}
			}
		}
	})
}

// Draw draws the current simulation frame to the screen
func Draw() {
	// don't bother drawing if we have a zero dimension
	if sz.WidthPx == 0 || sz.HeightPx == 0 {
		return
	}
	// on android in particular we need to avoid the status bar
	var topOffset float32
	if onAndroid {
		topOffset = float32(60) / sz.PixelsPerPt
	} else {
		topOffset = 0
	}
	// clear gl context
	glctx.ClearColor(0, 0, 0, 1)
	glctx.Clear(gl.COLOR_BUFFER_BIT)
	// determine letter boxing
	widthf := float32(img.RGBA.Bounds().Dx())
	heightf := float32(img.RGBA.Bounds().Dy())
	widthfSpace := float32(sz.WidthPt)
	heightfSpace := float32(sz.HeightPt) - topOffset
	ratioW := widthfSpace / widthf
	ratioH := (heightfSpace) / heightf
	var wpt geom.Pt
	var hpt geom.Pt
	if ratioW < ratioH {
		wpt = geom.Pt(widthf * ratioW)
		hpt = geom.Pt(heightf * ratioW)
	} else {
		wpt = geom.Pt(widthf * ratioH)
		hpt = geom.Pt(heightf * ratioH)
	}
	widthBorder := (geom.Pt(widthfSpace) - wpt) / 2
	heightBorder := (geom.Pt(heightfSpace)-hpt)/2 + geom.Pt(topOffset)
	// copy current simulation frame to opengl texture and display
	draw.Draw(img.RGBA, img.RGBA.Bounds(), sim.CurrentFrame, image.ZP, draw.Src)
	img.Upload()
	img.Draw(*sz,
		geom.Point{widthBorder, heightBorder},
		geom.Point{widthBorder + wpt, heightBorder},
		geom.Point{widthBorder, heightBorder + hpt},
		img.RGBA.Bounds())
}
