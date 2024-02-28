package main

import (
	"context"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"

	kuka "github.com/viam-soleng/viam-kuka/src"
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("Kuka Arm Go Module"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) (err error) {
	// instantiates the module itself
	myMod, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// Models and APIs add helpers to the registry during their init().
	// They can then be added to the module here.
	err = myMod.AddModelFromRegistry(ctx, arm.API, kuka.Model)
	if err != nil {
		return err
	}

	// Each module runs as its own process
	err = myMod.Start(ctx)
	defer myMod.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
