package main

import (
	"context"
	"fmt"
	"github.com/kytheron-org/kytheron/config"
	"github.com/kytheron-org/kytheron/kytheron"
	"github.com/kytheron-org/kytheron/registry"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"os"
)

var kytheronCmd = &cobra.Command{
	Use: "kytheron",
	Run: func(cmd *cobra.Command, args []string) {
		// Load our config
		configPath, _ := cmd.Flags().GetString("config")

		//policyPath, _ := cmd.Flags().GetString("policy")
		//
		//var policies []*policy.Policy
		//matches, err := filepath.Glob(policyPath)
		//if err != nil {
		//	log.Fatalf("Error globbing pattern %s: %v", policyPath, err)
		//}
		//for _, match := range matches {
		//	policyContents, err := ioutil.ReadFile(match)
		//	if err != nil {
		//		log.Fatal(err)
		//	}
		//	p, err := policy.Decode(match, policyContents)
		//	if err != nil {
		//		log.Fatal(err)
		//	}
		//	policies = append(policies, p)
		//}

		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(cfg.LogLevel)
		atom := zap.NewAtomicLevel()
		switch cfg.LogLevel {
		case "debug":
			atom.SetLevel(zapcore.DebugLevel)
		default:
			atom.SetLevel(zapcore.InfoLevel)
		}

		encoderConfig := zap.NewProductionEncoderConfig()
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			atom, // Use the AtomicLevel here
		)
		logger := zap.New(core)
		defer logger.Sync()

		pluginRegistry := registry.NewPluginRegistry(cfg.Registry.CacheDir)
		for _, plugin := range cfg.Plugins {
			if err := pluginRegistry.LoadPlugin(context.TODO(), plugin.Name, plugin.Version); err != nil {
				log.Fatal(err)
			}
		}

		k := kytheron.New(cfg, pluginRegistry, logger)
		if err := k.Run(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	kytheronCmd.Flags().StringP("config", "c", ".config.yaml", "path to config file")
	kytheronCmd.Flags().StringP("policy", "p", "", "path to policy file")
}

func main() {
	if err := kytheronCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
