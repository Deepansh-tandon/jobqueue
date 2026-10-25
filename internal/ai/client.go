package ai

import "jobqueue/internal/jobs"

type AI struct {
	Diagnoser jobs.Diagnoser
}

func New(d jobs.Diagnoser) *AI {
	if d == nil {
		d = jobs.NopDiagnoser{}
	}
	return &AI{Diagnoser: d}
}
