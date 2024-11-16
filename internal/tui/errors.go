package tui

type CmdError struct {
	reason string
}

func (e CmdError) Error() string {
	return e.reason
}
