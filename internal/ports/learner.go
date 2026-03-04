package ports

// AutotuneResult summarizes what RunMathTune changed.
type AutotuneResult struct {
	Promoted int // domains promoted context→core
	Demoted  int // domains demoted core→context
	Decayed  int // domains whose hits were decayed
	Pruned   int // domains removed (hits < PruneFloor)
}
