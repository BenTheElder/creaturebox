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
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/llgcode/draw2d/draw2dimg"
	"github.com/llgcode/draw2d/draw2dkit"
)

const (
	// The radius of the simulated "creature" and a float version
	creatureRadius  = 6
	creatureRadiusf = float64(creatureRadius)
	// The thickness of the moving obstacle lines:
	obstacleWidth = 3
	// The minimum number of creatures that must be alive:
	minCreatures = 10
	// The number of obstacles to spawn:
	numObstacles = 6
	// maxCreatures is the limit for automatically spawning creatures:
	maxCreatures = 2 * minCreatures
	// maxBestCreatures is the limit for the creature hall of fame for
	// evolution:
	maxBestCreatures = maxCreatures * 2
	// The number of simulation ticks between "evolution" spawning:
	evolutionCycleTicks = 30 * 5
)

var (
	// the background color of the simulation area
	BGColor = color.RGBA{0xF4, 0xF4, 0xF4, 0xFF}
	Black   = color.RGBA{0, 0, 0, 0xFF}
)

// Creature holds the state for a simulated "creature"
type Creature struct {
	x     float64
	y     float64
	angle float64
	score int64
	color color.Color
	brain *Brain
}

// GetAction returns the brain output for the creature at the current
// simulation state.
// WARNING: GetAction uses Sim.DistanceToNearest which relies on the state
// of the buffer. See Sim.DistanceToNearest
func (c *Creature) GetAction(s *Sim) (turn, move float64) {
	for i := 0; i < numBrainInputs; i++ {
		angle := math.Pi * 2 * float64(i) / numBrainInputs
		s.brainInputs[i] = s.DistanceToNearest(c, angle)
	}
	return c.brain.Step(s.brainInputs)
}

// Obstacle holds the state for simulated moving obstacle
type Obstacle struct {
	x      float64
	y      float64
	angle  float64
	dx     float64
	dy     float64
	length float64
}

// TopCreature is for tracking the Brain weights of Creatures
// with top scores.
type TopCreature struct {
	score   int64
	weights []float64
}

// WeightsEqual returns true if the the TopCreature's weights match
// the weights passed to WeightsEqual
func (t *TopCreature) WeightsEqual(weights []float64) bool {
	if len(t.weights) != len(weights) {
		return false
	}
	for i := 0; i < len(t.weights); i++ {
		if t.weights[i] != weights[i] {
			return false
		}
	}
	return true
}

// TopCreatures implements sort.Interface for []*TopCreature
type TopCreatures []*TopCreature

func (t TopCreatures) Len() int {
	return len(t)
}

func (t TopCreatures) Less(i, j int) bool {
	return t[i].score < t[j].score
}

func (t TopCreatures) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// IndexOfWeights returns the index at which the TopCreature with
// matching weights is found or -1 if the weights are not found
func (t TopCreatures) IndexOfWeights(weights []float64) int {
	for i := 0; i < len(t); i++ {
		if t[i].WeightsEqual(weights) {
			return i
		}
	}
	return -1
}

// Sim holds all of the simulation state
type Sim struct {
	width         int          // Simulation area width
	height        int          // Simulation area height
	borderWidth   int          // Simulation border thickness
	creatures     []*Creature  // The currently alive creatures
	creaturePool  []*Creature  // For recycling dead creatures
	obstacles     []Obstacle   // The moving obstacles
	bestCreatures TopCreatures // All time best brain patterns and scores
	CurrentFrame  *image.RGBA  // The framebuffer for drawing the simulation
	// The frame width, height, and border size as floats so we can cache them:
	frameWidthf  float64
	frameHeightf float64
	borderWidthf float64
	gc           *draw2dimg.GraphicContext // The draw2d context
	// Used for storing the inputs to the brain, as the size does not change
	// and we only ever need to do one brain at a time, no reason to keep
	// allocating this elsewhere.
	brainInputs []float64
	tickCounter int // For counting the number of elapsed ticks
}

// NewSim creates a new Sim with a worldsize (width, height)
// surrounded by a blank area of borderWidth
func NewSim(width, height, borderWidth int) *Sim {
	buffer := image.NewRGBA(image.Rect(0, 0, width+borderWidth*2, height+borderWidth*2))
	bounds := buffer.Bounds()
	gc := draw2dimg.NewGraphicContext(buffer)
	return &Sim{
		width:         width,
		height:        height,
		borderWidth:   borderWidth,
		creatures:     make([]*Creature, 0),
		creaturePool:  make([]*Creature, 0),
		obstacles:     make([]Obstacle, 0),
		bestCreatures: make(TopCreatures, 0),
		CurrentFrame:  buffer,
		frameWidthf:   float64(bounds.Dx()),
		frameHeightf:  float64(bounds.Dy()),
		borderWidthf:  float64(borderWidth),
		gc:            gc,
		brainInputs:   make([]float64, numBrainInputs),
		tickCounter:   0,
	}
}

// NewRandomCreature returns a new completely randomized Creature with a valid
// location within the simulation
func (s *Sim) NewRandomCreature() *Creature {
	b := NewRandomBrain()
	return &Creature{
		x:     float64(rand.Intn(s.width-creatureRadius) + creatureRadius),
		y:     float64(rand.Intn(s.height-creatureRadius) + creatureRadius),
		angle: rand.Float64() * 2 * math.Pi,
		color: b.GetColor(),
		brain: b,
	}
}

// NewRandomCreatureWithWeights returns a new randomized Creature with a brain
// from the provided weights and a valid location within the simulation
func (s *Sim) NewRandomCreatureWithWeights(weights []float64) *Creature {
	b := NewBrainFromWeights(weights)
	return &Creature{
		x:     float64(rand.Intn(s.width-creatureRadius) + creatureRadius),
		y:     float64(rand.Intn(s.height-creatureRadius) + creatureRadius),
		angle: rand.Float64() * 2 * math.Pi,
		color: b.GetColor(),
		brain: b,
	}
}

// NewRandomObstacle returns a new randomized obstacle with a valid location
// within the simulation
func (s *Sim) NewRandomObstacle() Obstacle {
	dx := rand.Float64()*2 - 1
	dy := rand.Float64()*2 - 1
	for dx == 0 {
		dx = rand.Float64()*2 - 1
	}
	for dy == 0 {
		dy = rand.Float64()*2 - 1
	}
	dx += math.Copysign(0.5, dx)
	dy += math.Copysign(0.5, dy)
	return Obstacle{
		x:      float64(rand.Intn(s.width)),
		y:      float64(rand.Intn(s.width)),
		angle:  rand.Float64() * 2 * math.Pi,
		dx:     dx,
		dy:     dy,
		length: float64(rand.Intn(s.width))/3 + float64(s.width)/6,
	}
}

// shuffleCreatures shuffles the creature list using the Durstenfeld
// version of the Fisher-Yates algorithm.
// See: https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
func (s *Sim) shuffleCreatures() {
	for i := len(s.creatures) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		s.creatures[i], s.creatures[j] = s.creatures[j], s.creatures[i]
	}
}

// SpawnRandomCreature adds a new random creature to the simulation,
// if possible it will re-initialize a creature from the creaturePool instead
// of allocating a new one.
func (s *Sim) SpawnRandomCreature() {
	lenCreaturePool := len(s.creaturePool)
	if lenCreaturePool > 0 {
		c := s.creaturePool[lenCreaturePool-1]
		c.brain.RandomizeWeights()
		c.color = c.brain.GetColor()
		c.x = float64(rand.Intn(s.width-creatureRadius) + creatureRadius)
		c.y = float64(rand.Intn(s.height-creatureRadius) + creatureRadius)
		c.angle = rand.Float64() * 2 * math.Pi
		s.creatures = append(s.creatures, c)
		s.creaturePool[lenCreaturePool-1] = nil
		s.creaturePool = s.creaturePool[:lenCreaturePool-1]
	} else {
		s.creatures = append(s.creatures, s.NewRandomCreature())
	}
}

// SpawnCreatureWithWeights adds a new random creature with a brain from the
// provided weights to the simulation, if possible it will re-initialize a
// creature from the creaturePool instead of allocating a new one.
func (s *Sim) SpawnCreatureWithWeights(weights []float64) {
	lenCreaturePool := len(s.creaturePool)
	if lenCreaturePool > 0 {
		c := s.creaturePool[lenCreaturePool-1]
		c.brain.SetWeights(weights)
		c.color = c.brain.GetColor()
		c.x = float64(rand.Intn(s.width-creatureRadius) + creatureRadius)
		c.y = float64(rand.Intn(s.height-creatureRadius) + creatureRadius)
		c.angle = rand.Float64() * 2 * math.Pi
		s.creatures = append(s.creatures, c)
		s.creaturePool[lenCreaturePool-1] = nil
		s.creaturePool = s.creaturePool[:lenCreaturePool-1]
	} else {
		s.creatures = append(s.creatures, s.NewRandomCreatureWithWeights(weights))
	}
}

// SpawnCreatures adds n new creatures to the simulation.
// 1/2 will be from the all time top scoring creature brain weights,
// 1/4 by combining two all time top scoring creatures' brain weights,
// the remaining 1/4 are purely random.
func (s *Sim) SpawnCreatures(n int) {
	i := 0
	// first try to spawn creatures based on the creature hall of fame
	lBestCreatures := len(s.bestCreatures)
	if lBestCreatures > 0 {
		lWeights := len(s.bestCreatures[0].weights)
		nToSpawn := n / 2
		if nToSpawn < 1 {
			nToSpawn = 1
		}
		// spawn copies of hall of famers
		for ; i < nToSpawn; i++ {
			weights := make([]float64, lWeights)
			copy(weights, s.bestCreatures[i%lBestCreatures].weights)
			s.SpawnCreatureWithWeights(weights)
		}
		if i == n {
			return
		}
		// spawn mixed versions
		for offset := 0; i < n/4; i++ {
			weights := make([]float64, lWeights)
			j := 0
			divider := rand.Intn(lWeights)
			for ; j < divider; j++ {
				weights[j] = s.bestCreatures[(offset)%lBestCreatures].weights[j]
			}
			for ; j < lWeights; j++ {
				weights[j] = s.bestCreatures[(offset+1)%lBestCreatures].weights[j]
			}
			s.SpawnCreatureWithWeights(weights)
			offset++
		}
	}
	// now spawn random creatures for the remainder
	for ; i < n; i++ {
		s.SpawnRandomCreature()
	}
}

// SpawnObstacles adds n new random obstacles to the simulation
func (s *Sim) SpawnObstacles(n int) {
	for i := 0; i < n; i++ {
		s.obstacles = append(s.obstacles, s.NewRandomObstacle())
	}
}

// xyDist returns the distance from (x,y) to (p,q)
func xyDist(x, y, p, q float64) float64 {
	return math.Sqrt(math.Pow((x-p), 2) + math.Pow((y-q), 2))
}

// DistanceToNearest returns the distance to closest obstacle in along
// the ray cast from the creature's forward direction rotated by angle.
//
// WARNING: DistanceToNearest assumes the buffer has been cleared and the
// simulation excluding the creatures has been drawn for the current state.
// We raycast by exploring the ray's path through the buffer looking for a
// non-background-color pixel.
func (s *Sim) DistanceToNearest(c *Creature, angle float64) float64 {
	dist := math.MaxFloat64
	ax := math.Cos(angle + c.angle)
	ay := math.Sin(angle + c.angle)
	x := c.x + ax
	y := c.y + ay
	for x < s.frameWidthf && x >= 0 && y < s.frameHeightf && y >= 0 {
		if s.CurrentFrame.At(int(x), int(y)) != BGColor {
			dist = xyDist(c.x, c.y, x, y)
			break
		}
		x += ax
		y += ay
	}
	return dist
}

// DoTick runs the simulation by a single tick including drawing the new frame
// to s.CurrentFrame
func (s *Sim) DoTick() {
	// draw the sim border
	s.gc.SetFillColor(Black)
	// top
	draw2dkit.Rectangle(s.gc, 0, 0, s.frameWidthf, s.borderWidthf)
	s.gc.Fill()
	// left
	draw2dkit.Rectangle(s.gc, 0, s.borderWidthf, s.borderWidthf, s.frameHeightf)
	s.gc.Fill()
	// right
	draw2dkit.Rectangle(s.gc, s.frameWidthf-s.borderWidthf, s.borderWidthf,
		s.frameWidthf, s.frameHeightf)
	s.gc.Fill()
	// bottom
	draw2dkit.Rectangle(s.gc, s.borderWidthf, s.frameHeightf-s.borderWidthf,
		s.frameWidthf-s.borderWidthf, s.frameHeightf)
	s.gc.Fill()

	// clear actual drawing area to BG color
	draw2dkit.Rectangle(s.gc, s.borderWidthf, s.borderWidthf,
		s.frameWidthf-s.borderWidthf, s.frameHeightf-s.borderWidthf)
	s.gc.SetFillColor(BGColor)
	s.gc.Fill()

	// update Obstacles
	for i := 0; i < len(s.obstacles); i++ {
		s.obstacles[i].x += s.obstacles[i].dx
		s.obstacles[i].y += s.obstacles[i].dy
		// remove obstacles far off screen
		x := s.obstacles[i].x
		y := s.obstacles[i].y
		l := s.obstacles[i].length
		if (x-s.frameWidthf) >= l || -x >= l ||
			(y-s.frameHeightf) >= l || -y >= l {
			s.obstacles = append(s.obstacles[:i], s.obstacles[i+1:]...)
			i--
		}
	}
	if len(s.obstacles) < numObstacles {
		s.SpawnObstacles(numObstacles - len(s.obstacles))
	}

	// draw Obstacles
	s.gc.SetFillColor(color.Black)
	s.gc.SetLineWidth(obstacleWidth)
	for i := 0; i < len(s.obstacles); i++ {
		x := s.obstacles[i].x
		y := s.obstacles[i].y
		l := s.obstacles[i].length
		a := s.obstacles[i].angle
		s.gc.MoveTo(s.borderWidthf+x, s.borderWidthf+y)
		s.gc.LineTo(s.borderWidthf+x+math.Cos(a)*l, s.borderWidthf+y+math.Sin(a)*l)
		s.gc.FillStroke()
		s.gc.Close()
	}

	// handle evolution cycle
	if s.tickCounter%evolutionCycleTicks == 0 {
		// spawn new creatures if we aren't already overpopulated
		if len(s.creatures) < maxCreatures {
			s.SpawnCreatures(maxCreatures - len(s.creatures))
		}
	}

	// spawn new creatures if we have less than minimum
	if len(s.creatures) < minCreatures {
		s.SpawnCreatures(minCreatures - len(s.creatures))
	}
	// randomize creature order
	s.shuffleCreatures()

	// first remove "dead" creatures
	for i := 0; i < len(s.creatures); i++ {
		// determine bounding box
		x := s.borderWidthf + s.creatures[i].x
		y := s.borderWidthf + s.creatures[i].y
		left := int(x) - creatureRadius
		top := int(y) - creatureRadius
		right := int(x) + creatureRadius
		bottom := int(y) + creatureRadius
		dead := false
		for cy := top; cy <= bottom && !dead; cy++ {
			for cx := left; cx <= right && !dead; cx++ {
				if xyDist(x, y, float64(cx), float64(cy)) <= creatureRadiusf {
					if s.CurrentFrame.At(cx, cy) != BGColor {
						dead = true
					}
				}
			}
		}
		// if dead, remove
		if dead {
			weights := s.creatures[i].brain.GetWeights()
			index := s.bestCreatures.IndexOfWeights(weights)
			if index == -1 {
				s.bestCreatures = append(s.bestCreatures, &TopCreature{
					weights: s.creatures[i].brain.GetWeights(),
					score:   s.creatures[i].score,
				})
			} else {
				if s.bestCreatures[index].score < s.creatures[i].score {
					s.bestCreatures[index].score = s.creatures[i].score
				}
			}
			s.creaturePool = append(s.creaturePool, s.creatures[i])
			s.creatures, s.creatures[len(s.creatures)-1] =
				append(s.creatures[:i], s.creatures[i+1:]...), nil
			i--
		}
	}

	// update each creature
	for i := range s.creatures {
		turn, move := s.creatures[i].GetAction(s)
		//move = (move + 1) / float64(2)
		s.creatures[i].angle += turn / 8
		ax := math.Cos(s.creatures[i].angle)
		ay := math.Sin(s.creatures[i].angle)
		s.creatures[i].x += ax * move * 4 /*+ ax*math.Copysign(turn/2, move)*/
		s.creatures[i].y += ay * move * 4 /*+ ay*math.Copysign(turn/2, move)*/
		// increment the score unless somehow we've reached the maximum score
		if s.creatures[i].score < math.MaxInt64 {
			s.creatures[i].score++
		}
	}

	// update top creatures
	for i := 0; i < len(s.creatures); i++ {
		weights := s.creatures[i].brain.GetWeights()
		index := s.bestCreatures.IndexOfWeights(weights)
		if index == -1 {
			s.bestCreatures = append(s.bestCreatures, &TopCreature{
				weights: s.creatures[i].brain.GetWeights(),
				score:   s.creatures[i].score,
			})
		} else {
			if s.bestCreatures[index].score < s.creatures[i].score {
				s.bestCreatures[index].score = s.creatures[i].score
			}
		}
	}
	// sort top creatures
	sort.Sort(sort.Reverse(s.bestCreatures))
	// remove excess top creatures
	if len(s.bestCreatures) > maxBestCreatures {
		for i := len(s.bestCreatures) - 1; i > maxCreatures; i-- {
			s.bestCreatures[i] = nil
			s.bestCreatures = s.bestCreatures[:len(s.bestCreatures)-1]
		}
	}

	// draw creatures
	for i := range s.creatures {
		s.gc.SetFillColor(s.creatures[i].color)
		draw2dkit.Circle(s.gc, s.borderWidthf+s.creatures[i].x,
			s.borderWidthf+s.creatures[i].y, creatureRadius)
		s.gc.Fill()
		ax := math.Cos(s.creatures[i].angle)
		ay := math.Sin(s.creatures[i].angle)
		s.gc.SetFillColor(color.White)
		draw2dkit.Circle(s.gc, s.borderWidthf+s.creatures[i].x+ax*3,
			s.borderWidthf+s.creatures[i].y+ay*3, 2)
		s.gc.Fill()
	}

	// increment tick count
	s.tickCounter++
}
