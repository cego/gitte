package internal

import (
	"context"
	"fmt"
	"gitte/executor"
)

type actionOutputHandler struct {
}

func NewActionOutputHandler() *actionOutputHandler {
	return &actionOutputHandler{}
}

func (oh *actionOutputHandler) HandleOutput(ctx context.Context, output executor.Output) error {
	prefix := fmt.Sprintf("[%s][%s]: ", output.CmdName, output.Stream)
	fmt.Printf("%s%s", prefix, string(output.Output))
	return nil
}
