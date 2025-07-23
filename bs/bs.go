package main

import (
	sbbs "github.com/barbell-math/smoothbrain-bs"
)

func main() {
	sbbs.RegisterBsBuildTarget()
	sbbs.RegisterUpdateDepsTarget()
	sbbs.RegisterGoMarkDocTargets()
	sbbs.RegisterCommonGoCmdTargets(sbbs.NewGoTargets().
		DefaultFmtTarget().
		DefaultTestTarget(),
	)
	sbbs.RegisterMergegateTarget(sbbs.MergegateTargets{
		CheckDepsUpdated:     true,
		CheckReadmeGomarkdoc: true,
		FmtTarget:            sbbs.DefaultFmtTargetName,
		TestTarget:           sbbs.DefaultTestTargetName,
	})
	sbbs.Main("bs")
}
