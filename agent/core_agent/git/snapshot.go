package git

import "matrixops-agent/snapshot"

func RestoreSnapshot(projectID, directory, hash string, clean bool) error {
	return snapshot.Restore(projectID, directory, hash, clean)
}
