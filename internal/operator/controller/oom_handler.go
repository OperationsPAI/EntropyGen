package controller

// expandMemoryLimit computes the new memory limit after OOMKilled.
// Rules: x1.5, round up to 128Mi boundary, cap at max(2*initial, 2048Mi).
// Returns (newMi, shouldExpand).
func expandMemoryLimit(currentMi, initialMi int64) (int64, bool) {
	const minCeilMi = 128
	const absoluteMaxMi = 2048

	maxMi := initialMi * 2
	if maxMi < absoluteMaxMi {
		maxMi = absoluteMaxMi
	}
	if currentMi >= maxMi {
		return currentMi, false
	}

	newMi := currentMi * 3 / 2
	// Round up to 128Mi boundary
	newMi = ((newMi + minCeilMi - 1) / minCeilMi) * minCeilMi
	if newMi > maxMi {
		newMi = maxMi
	}
	return newMi, true
}
