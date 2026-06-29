//go:build !windows
// +build !windows

package backend

func killResidualRuntimeProcesses(appRoot string, dataRoot string) error {
	return nil
}
