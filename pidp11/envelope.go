package pidp11

import (
	"fmt"
	"math"
)

// A stage describes a linear progression between 2 brightness levels
type stage struct {
	loops     int  // duration of the stage, in loops
	start     int  // initial brightness
	end       int  // final brightness
	stepLoops int  // when to increase/decrease brightness, 0 if bright == target
	final     bool // is the stage is final, meaning brightness will remain on target
}

func (s stage) String() string {
	return fmt.Sprintf("stage[loops=%d, bright=%d→%d, stepLoops=%d, final=%v]",
		s.loops, s.start, s.end, s.stepLoops, s.final)
}

// The envelope describes the evolution of brightness via a sequence of stages
type envelope struct {
	stages   [4]stage // size must accomodate our longest envelope
	offset   int      // in the current stage, in loops
	stageNum int      // index of the current stage in the stages array
	count    int      // number of stages
}

func (env *envelope) String() string {
	return fmt.Sprintf("envelope[stages=%v, offset=%d, index=%d, count=%d]",
		env.stages[:env.count], env.offset, env.stageNum, env.count)
}

// Clear all stages, reset offset
func (env *envelope) reset() {
	env.offset = 0
	env.stageNum = 0
	env.count = 0
}

func (env *envelope) isPeriodic() bool {
	// xxx assumes all one-shot envelopes have only 1 stage
	return env.count > 1
}

func (env *envelope) addStage(bright1, bright2, ms int, final bool) {
	debug("addStage: %d→%d, ms=%d, final=%v", bright1, bright2, ms, final)
	assert(env.count < len(env.stages),
		"too many stages on envelope %v", env)
	loops := msToLoops(ms)
	stepLoops := 0
	if bright1 != bright2 {
		delta := abs(bright1 - bright2)
		stepLoops = loops / delta
		if stepLoops == 0 || stepLoops*delta > loops {
			// We don't have time for gradual change
			loops = 0
			stepLoops = 0
			bright1 = bright2
		}
	}
	env.stages[env.count] = stage{
		loops:     loops,
		start:     bright1,
		end:       bright2,
		stepLoops: stepLoops,
		final:     final,
	}
	env.count++
}

func (spec *ledSpec) makeEnvelope(fx Effect, brightP float64, params ...float64) {
	env := &spec.env
	env.reset()
	bright := int(math.Round(brightP * (BRIGHTNESS_STEPS - 1)))
	fx.makeEnvelope(spec, bright, params...)
}

// Advance by one step through the envelope and set brightness
func (spec *ledSpec) step() {
	spec.Lock()
	defer spec.Unlock()

	env := &spec.env
	if env.count == 0 { // fixed brightness
		return
	}
	stage := env.stages[env.stageNum]
	if env.offset == stage.loops { // End of stage?
		if stage.final {
			// Remove stage and remain forever on current brightness
			spec.bright = stage.end
			env.count = 0
			env.stageNum = 0
		} else {
			// Move to the next stage
			env.stageNum = (env.stageNum + 1) % env.count
			env.offset = 0
			spec.bright = env.stages[env.stageNum].start
		}
	} else { // Progress through stage
		if stage.stepLoops != 0 && env.offset%stage.stepLoops == 0 {
			if spec.bright < stage.end {
				spec.bright++
			} else if spec.bright > 0 {
				spec.bright--
			}
		}
		env.offset++
	}
}

// Returns a [0, 1] cursor indicating progress through the envelope
// This is required for periodic effects: If the frequency of the calls
// to SetLed() is higher than the frequency param, starting the envelope
// from 0 on every call would result in a incorrect high visual
// frequency, as well as jarring visual irregularities.
// So we smooth out the transition by tracking our relative location
// through the envelope.
func (spec *ledSpec) getProgress() float64 {
	env := &spec.env
	if !env.isPeriodic() {
		return 0
	}
	totalLoops := 0
	totalOffset := 0
	for i := 0; i < env.count; i++ {
		stageLoops := env.stages[i].loops
		switch {
		case i < env.stageNum:
			totalOffset += stageLoops
		case i == env.stageNum:
			totalOffset += env.offset
		}
		totalLoops += stageLoops
	}
	return float64(totalOffset) / float64(totalLoops)
}

// Set the envelope's stage, offset, set corresponding brigthness
func (spec *ledSpec) setProgress(pct float64) {
	env := &spec.env
	if !env.isPeriodic() {
		spec.bright = env.stages[0].start
		return
	}
	totalLoops := 0
	for i := 0; i < env.count; i++ {
		stageLoops := env.stages[i].loops
		totalLoops += stageLoops
	}
	loops := int(math.Round(pct * float64(totalLoops)))
	sofar := 0
	var stage, offset, bright int
	for i := 0; i < env.count; i++ {
		s := &env.stages[i]
		if loops < sofar+s.loops {
			stage = i
			offset = loops - sofar
			bright = scale(offset, s.loops, s.start, s.end)
			break
		}
		sofar += s.loops
	}
	env.stageNum = stage
	env.offset = offset
	spec.bright = bright
}

func msToLoops(ms int) int {
	return int(math.Round(float64(ms) * 1000 / float64(loopμs)))
}

func abs(x int) int {
	if x < 0 {
		return -x
	} else {
		return x
	}
}
