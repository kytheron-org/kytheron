package kytheron

import (
	"github.com/kytheron-org/kytheron/config"
	"github.com/kytheron-org/kytheron/policy"
	"github.com/kytheron-org/kytheron/registry"
	"go.uber.org/zap"
	"log"
)

// What does our class do
// It has to maintain a buffer of recent logs
// For now, let's just give it a list of policies
// We'll store a map of sources, and the policies that need to be evaluated

// TODO: We'll want instrumentation as well
// - how many log lines its processed
// - the lag between current message, and tail of the stream
type Kytheron struct {
	Policies       map[string]*policy.Policy
	mappedPolicies map[string]map[string][]string
	config         *config.Config
	pluginRegistry *registry.PluginRegistry
	logger         *zap.Logger
}

func New(cfg *config.Config, pluginRegistry *registry.PluginRegistry, logger *zap.Logger) *Kytheron {
	k := &Kytheron{
		Policies:       make(map[string]*policy.Policy),
		mappedPolicies: make(map[string]map[string][]string),
		pluginRegistry: pluginRegistry,
		config:         cfg,
		logger:         logger,
	}
	//for _, policy := range policies {
	//	k.Policies[policy.Name] = policy
	//}
	k.Init()
	return k
}

func (k *Kytheron) Init() {
	for _, policy := range k.Policies {
		for _, input := range policy.Sources {
			if _, ok := k.mappedPolicies[input.Type]; !ok {
				k.mappedPolicies[input.Type] = map[string][]string{}
			}
			if _, ok := k.mappedPolicies[input.Type][input.Name]; !ok {
				k.mappedPolicies[input.Type][input.Name] = []string{}
			}
			k.mappedPolicies[input.Type][input.Name] = append(k.mappedPolicies[input.Type][input.Name], policy.Name)
		}

		for _, output := range policy.Outputs {
			if output.Version != "" {

			}
		}
	}
}

//func (k *Kytheron) Handle(log *log.Log) error {
//	inputName := log.Source.Name
//	inputType := log.Source.Type
//	for _, policyName := range k.mappedPolicies[inputType][inputName] {
//		//fmt.Printf("Executing policy %s\n", policyName)
//		policy := k.Policies[policyName]
//		if err := policy.Process(log); err != nil {
//			fmt.Println(err)
//			return err
//		}
//	}
//	return nil
//}

func (k *Kytheron) Run() error {
	srv := &GrpcServer{logger: k.logger}

	go func() {
		if err := NewProcessor(k.config, k.pluginRegistry, k.logger).Run(); err != nil {
			log.Fatal(err)
		}
	}()

	return srv.Start(k.config)
}
