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
	"image/color"
	"math"
	"math/rand"
)

const (
	// the number of sensor inputs for the brain
	numBrainInputs = 12
	// the number of outputs to feed back as inputs to the brain
	memorySize = 12
)

// Perceptron is a simple perceptron
type Perceptron struct {
	weights []float64
}

// NewPerceptron creates a new perceptron with the provided weights
func NewPerceptron(weights []float64) Perceptron {
	return Perceptron{
		weights: weights,
	}
}

// TanhActivation calculates the ouput for a given input vector
// using the hyperbolic tangent function for activation
func (p *Perceptron) TanhActivation(inputs []float64) float64 {
	sum := float64(0)
	for k := 0; k < len(inputs); k++ {
		sum += p.weights[k] * inputs[k]
	}
	return math.Tanh(sum)
}

// Brain is a simple recurrent two layer neural network.
// The last output is stored for use as input to the next
// step.
// This has a silly little architecture for use with the
// simulated "Creatures".
type Brain struct {
	inLayer    []Perceptron
	outLayer   []Perceptron
	output     []float64
	x          []float64
	inOutput   []float64
	allWeights []float64
}

// NewBrainRandomized creates a new "Brain" with randomized
// weights. The input layer takes numBrainInputs inputs (see sim.go)
// and memorySize outputs from the previous output (simple rnn)
// output layer has memorySize + 2 nodes where the first
// two output nodes are used for control (actual output)
// and the remainder are used for memory.
func NewRandomBrain() *Brain {
	b := Brain{
		inLayer:  make([]Perceptron, numBrainInputs+memorySize),
		outLayer: make([]Perceptron, memorySize+2),
		output:   make([]float64, memorySize+2),
		// we preallocate the input + bias + memory slice
		x: make([]float64, numBrainInputs+memorySize+1),
		// we also preallocate the inLayer output
		inOutput: make([]float64, numBrainInputs+memorySize+1),
	}
	// we will have numBrainInputs+memorySize inputs and a bias
	inWeightLen := numBrainInputs + memorySize + 1
	// we will have len(inLayer) inputs and a bias
	outWeightLen := len(b.inLayer) + 1
	l := len(b.inLayer)*inWeightLen + len(b.outLayer)*outWeightLen
	b.allWeights = make([]float64, l)
	// These should always be 1 for bias
	b.x[0] = 1
	b.inOutput[0] = 1
	// initialize Perceptrons
	for i := range b.inLayer {
		offset := i * inWeightLen
		for j := 0; j < inWeightLen; j++ {
			b.allWeights[offset+j] = rand.Float64()*2 - 1
		}
		b.inLayer[i] = NewPerceptron(b.allWeights[offset : offset+inWeightLen])
	}
	for i := range b.outLayer {
		offset := len(b.inLayer)*inWeightLen + outWeightLen*i
		for j := 0; j < outWeightLen; j++ {
			b.allWeights[offset+j] = rand.Float64()*2 - 1
		}
		b.outLayer[i] = NewPerceptron(b.allWeights[offset : offset+outWeightLen])
	}
	// initialize output to zero
	for i := range b.output {
		b.output[i] = 0
	}
	return &b
}

// NewBrainFromWeights creates a new brain from a slize of weights
// like those returned from GetWeights()
func NewBrainFromWeights(weights []float64) *Brain {
	b := Brain{
		inLayer:  make([]Perceptron, numBrainInputs+memorySize),
		outLayer: make([]Perceptron, memorySize+2),
		output:   make([]float64, memorySize+2),
		// we preallocate the input + bias + memory slice
		x: make([]float64, numBrainInputs+memorySize+1),
		// we also preallocate the inLayer output
		inOutput:   make([]float64, numBrainInputs+memorySize+1),
		allWeights: weights,
	}
	// These should always be 1 for bias
	b.x[0] = 1
	b.inOutput[0] = 1
	// initialize Perceptrons
	// we will have numBrainInputs+memorySize inputs and a bias
	inWeightLen := numBrainInputs + memorySize + 1
	inLayerLen := len(b.inLayer)
	// we will have len(inLayer) inputs and a bias
	outWeightLen := inLayerLen + 1
	for i := range b.inLayer {
		offset := i * inWeightLen
		b.inLayer[i] = NewPerceptron(b.allWeights[offset : offset+inWeightLen])
	}
	for i := range b.outLayer {
		offset := inLayerLen*inWeightLen + outWeightLen*i
		b.outLayer[i] = NewPerceptron(b.allWeights[offset : offset+outWeightLen])
	}
	// initialize output to zero
	for i := range b.output {
		b.output[i] = 0
	}
	return &b
}

// Step computes the output by computing the feed forward of
// the input fed to the current network state. This has the
// side-effect of updating the stored output to be used for the
// next step.
func (b *Brain) Step(input []float64) (turn, move float64) {
	if len(input) != numBrainInputs {
		panic("input length is wrong!")
	}
	// setup input vector
	// actual input
	j := 0
	for ; j < len(input); j++ {
		b.x[j+1] = input[j]
	}
	j -= 2
	// last output[2..]
	for i := 2; i < len(b.output); i++ {
		b.x[j+i] = b.output[i]
	}
	for i := 0; i < len(b.inLayer); i++ {
		b.inOutput[i+1] = b.inLayer[i].TanhActivation(b.x)
	}
	// compute output of outLayer
	for i := 0; i < len(b.outLayer); i++ {
		b.output[i] = b.outLayer[i].TanhActivation(b.inOutput)
	}
	return b.output[0], b.output[1]
}

// GetWeights returns a slice of all weights in the brain
// The slice contains the weights in order of inLayer first then
// outLayer and within each layer the weights of the Perceptrons
// are ordered as they are in the brain.
func (b *Brain) GetWeights() (allWeights []float64) {
	return b.allWeights
}

// SetWeights the counterpart to GetWeights
func (b *Brain) SetWeights(allWeights []float64) {
	b.allWeights = allWeights
	// initialize Perceptrons
	// we will have numBrainInputs+memorySize inputs and a bias
	inWeightLen := numBrainInputs + memorySize + 1
	inLayerLen := len(b.inLayer)
	// we will have len(inLayer) inputs and a bias
	outWeightLen := inLayerLen + 1
	for i := range b.inLayer {
		offset := i * inWeightLen
		b.inLayer[i] = NewPerceptron(b.allWeights[offset : offset+inWeightLen])
	}
	for i := range b.outLayer {
		offset := inLayerLen*inWeightLen + outWeightLen*i
		b.outLayer[i] = NewPerceptron(b.allWeights[offset : offset+outWeightLen])
	}
}

// RandomizeWeights randomizes the Brain's weights
func (b *Brain) RandomizeWeights() {
	lenAllWeights := len(b.allWeights)
	for i := 0; i < lenAllWeights; i++ {
		b.allWeights[i] = rand.Float64()*2 - 1
	}
}

// Compute a color based on the brain's weights
func (b *Brain) GetColor() color.RGBA {
	lenAllWeights := len(b.allWeights)
	// length of each section to compute
	lenAllWeights_div_3 := lenAllWeights / 3
	// float version for calculations
	lenAllWeights_div_3f := float64(lenAllWeights_div_3)
	// compute an average of each 1/3 of the brain to use for
	// the red, green, blue color channels
	redAvg := float64(0)
	greenAvg := float64(0)
	blueAvg := float64(0)
	for i := 0; i < lenAllWeights; i++ {
		if i < lenAllWeights_div_3 {
			redAvg += b.allWeights[i] / lenAllWeights_div_3f
		} else if i < lenAllWeights_div_3*2 {
			blueAvg += b.allWeights[i] / lenAllWeights_div_3f
		} else {
			greenAvg += b.allWeights[i] / lenAllWeights_div_3f
		}
	}
	// we want only positive ranges, with nice colors
	redAvg = float64(1) / ((redAvg * 2) + 0.5)
	greenAvg = float64(1) / ((greenAvg * 2) + 0.5)
	blueAvg = float64(1) / ((blueAvg * 2) + 0.5)
	c := color.RGBA{
		// mask needs to be less than 0xFF because we don't want any white creatures
		uint8(0xB3 * redAvg),
		uint8(0xB3 * greenAvg),
		uint8(0xB3 * blueAvg),
		// Fully opaque
		0xFF,
	}
	return c
}
