package main

import (
	"flag"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flags flag.FlagSet
	var logLevel zapcore.Level
	flags.Var(&logLevel, "log_level", "log level")
	templateDir := flags.String("templates", "", "path to custom templates")
	verboseMode := flags.Bool("verbose", false, "enable verbose mode")
	continueOnError := flags.Bool("continue_on_error", false, "continue on error")
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}
	opts.Run(func(p *protogen.Plugin) error {
		return NewGenerator(Options{
			TemplateDir:     *templateDir,
			Verbose:         *verboseMode,
			ContinueOnError: *continueOnError,
			Logger:          setupLogger(logLevel),
		}).Generate(p)
	})
}

func setupLogger(logLevel zapcore.Level) *zap.Logger {
	// set up a ap logger and have it write to stderr:
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(logLevel)
	config.OutputPaths = []string{"stderr"}
	log, _ := config.Build()
	return log

}
